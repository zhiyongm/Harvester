package main

import (
	"blockcooker/api"
	"blockcooker/chain"
	"blockcooker/core"
	p2p "blockcooker/network"
	"blockcooker/resultWriter"
	"blockcooker/txGenerator"
	"blockcooker/txInjector"
	"blockcooker/utils"
	"context"
	"flag"
	_ "flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

func initLog() {

}
func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	var configPath = flag.String("c", "config_leader.json", "path to config file")
	flag.Parse()

	config := utils.NewConfigFromJson(*configPath)

	if config.DropDatabase {
		if config.StoragePath != "/" {
			lvldbPath := filepath.Join(config.StoragePath, "levelDB_node_"+strconv.FormatUint(config.NodeID, 10))
			os.RemoveAll(lvldbPath)
		}
	}

	exitChan := make(chan os.Signal)
	signal.Notify(exitChan, os.Interrupt, os.Kill, syscall.SIGTERM)

	if config.NodeMode == "leader" {
		initLog()
		log.Info("启动出块节点模式")
		go startLeader(ctx, config)
	}

	if config.NodeMode == "peer" {
		initLog()
		log.Info("启动同步节点模式 Peer node mode")
		go startPeer(ctx, config)
	}

	if config.NodeMode != "leader" && config.NodeMode != "peer" {
		log.Error("无法识别的节点模式，请检查配置文件中的 node_mode 设置。 Unrecognized node mode, please check the node_mode setting in the configuration file.")
		os.Exit(1)
	}

	for {
		select {
		case sig := <-exitChan:
			fmt.Println("进程停止：", sig)
			cancel()
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}
	}
}

func startLeader(ctx context.Context, cc *utils.Config) {
	bc, _ := chain.NewBlockChain(cc)
	api_server := api.NewAPI(bc)
	go api_server.Start(cc.APIPort)
	log.Info("区块链初始化完成，开始交易注入")
	var txg txGenerator.TxGeneratorInterface
	if cc.TxGeneratorMode == "random" {
		txg = txGenerator.NewTxGeneratorFromRandom()
	}
	if cc.TxGeneratorMode == "csv" {
		txg = txGenerator.NewTxGeneratorFromCSV(cc.InputDatasetPath)
	}
	if txg == nil {
		log.Error("交易生成器初始化失败，请检查配置文件中的 tx_generator_mode 设置。")
		os.Exit(1)
	}
	log.Info("交易注入者节点启动成功，开始注入交易")

	tx_injector := txInjector.NewTxInjector(bc)
	go tx_injector.StartTxInjector(cc)

	time.Sleep(5 * time.Second)
	log.Info("交易池准备就绪，开始区块生成")

	log.Info("启动P2P发布者")
	p2pPublisher, err := p2p.NewPublisher(ctx, cc.P2PTopic, cc.P2PSenderIDFilePath, cc.P2PSenderListenAddress)
	if err != nil {
		log.Error("P2P发布者初始化失败，1秒后重试：", err)
		time.Sleep(1 * time.Second)
	}

	for {
		if len(p2pPublisher.Topic.ListPeers()) >= cc.PeerCount {
			log.Info("P2P节点数量达到要求，开始区块生成，当前连接数：", len(p2pPublisher.Topic.ListPeers()), "/", cc.PeerCount)
			break
		} else {
			log.Info("等待P2P连接，当前连接数：", len(p2pPublisher.Topic.ListPeers()), "/", cc.PeerCount)
			time.Sleep(1 * time.Second)
		}
	}

	go bc.StartBlockGenerator(ctx, p2pPublisher)

}

func startPeer(ctx context.Context, cc *utils.Config) {
	bc, _ := chain.NewBlockChain(cc)
	api_server := api.NewAPI(bc)
	go api_server.Start(cc.APIPort)
	log.Info("区块链初始化完成，开始同步区块")

	go bc.StartBlockAdder(ctx, nil)

	log.Info("启动P2P接收者")

	var p2pSubscriber *p2p.Subscriber
	for {
		var err error
		p2pSubscriber, err = p2p.NewSubscriber(ctx, cc.P2PTopic, cc.P2PSenderIDFilePath, cc.ProposerAddress)
		if err != nil {
			log.Error("P2P订阅者初始化失败，1秒后重试：", err)
			time.Sleep(1 * time.Second)
			continue
		}
		log.Info("P2P订阅者初始化成功，开始接收区块")
		break
	}

	for {
		log.Info("开始接收区块")
		msg, err := p2pSubscriber.Sub.Next(ctx)
		if err != nil {
			log.Error("接收消息失败", err)
		}

		if msg.GetFrom() == p2pSubscriber.Host.ID() {
			continue
		}

		block_message_size := len(msg.GetData())
		fmt.Printf("Subscriber %s received message from %s Size: %.2f MB \n", p2pSubscriber.Host.ID(), msg.GetFrom(), float64(block_message_size)/1024/1024)

		blk := core.DecodeB(msg.GetData())

		bc.ResultWriter.CsvEntityChan <- &resultWriter.ResultEntity{BlockNumber: blk.Header.Number, MessageSize: block_message_size}
		log.Info("Recieved a block No", blk.Header.Number, "ON", time.Now())
		bc.ToBePackagedBlocks <- blk
	}
}
