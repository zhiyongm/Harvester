package p2p

import (
	"context"
	"fmt"
	"os"

	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Publisher struct {
	host  host.Host
	ps    *pubsub.PubSub
	Topic *pubsub.Topic
}

type Subscriber struct {
	Host host.Host
	ps   *pubsub.PubSub
	Sub  *pubsub.Subscription
}

func loadOrCreatePrivateKey(keyFilePath string) (crypto.PrivKey, error) {
	if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
		privKey, _, err := crypto.GenerateEd25519Key(nil)
		log.Info("Private key file not found, generating a new one.")
		if err != nil {
			return nil, fmt.Errorf("failed to generate private key: %w", err)
		}

		keyBytes, err := crypto.MarshalPrivateKey(privKey)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal private key: %w", err)
		}

		if err := os.WriteFile(keyFilePath, keyBytes, 0600); err != nil {
			return nil, fmt.Errorf("failed to write private key to file: %w", err)
		}
		log.Printf("Generated and saved new private key to: %s", keyFilePath)
		return privKey, nil
	}

	keyBytes, err := os.ReadFile(keyFilePath)
	log.Info("Private key file found.")
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	privKey, err := crypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	log.Printf("Loaded existing private key from: %s", keyFilePath)
	return privKey, nil
}

func NewPublisher(ctx context.Context, topicName, keyFilePath string, P2PSenderListenAddress string) (*Publisher, error) {
	privKey, err := loadOrCreatePrivateKey(keyFilePath)
	if err != nil {
		return nil, err
	}

	maxMessageSize := 100 * 1024 * 1024

	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(P2PSenderListenAddress),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher host: %w", err)
	}

	ps, err := pubsub.NewGossipSub(ctx, h, pubsub.WithMaxMessageSize(maxMessageSize))
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub service: %w", err)
	}

	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join Topic: %w", err)
	}

	return &Publisher{
		host:  h,
		ps:    ps,
		Topic: topic,
	}, nil
}

func (p *Publisher) PublishMessage(ctx context.Context, message string) error {
	return p.Topic.Publish(ctx, []byte(message))
}

func NewSubscriber(ctx context.Context, topicName string, keyFilePath string, publisher_IP string) (*Subscriber, error) {

	PublisherPrivKey, err := loadOrCreatePrivateKey(keyFilePath)
	publisher_h, err := libp2p.New(
		libp2p.Identity(PublisherPrivKey),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
	)

	Addrs, _ := multiaddr.NewMultiaddr(publisher_IP)

	publisher_Info := peer.AddrInfo{
		ID:    publisher_h.ID(),
		Addrs: []multiaddr.Multiaddr{Addrs},
	}

	if err != nil {
		return nil, err
	}

	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create subscriber host: %w", err)
	}
	maxMessageSize := 100 * 1024 * 1024
	ps, err := pubsub.NewGossipSub(ctx, h, pubsub.WithMaxMessageSize(maxMessageSize))
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub service: %w", err)
	}

	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to join Topic: %w", err)
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to Topic: %w", err)
	}

	err = ConnectToPeer(ctx, h, publisher_Info)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to publisher: %w", err)
	}
	return &Subscriber{
		Host: h,
		ps:   ps,
		Sub:  sub,
	}, nil
}

func getPublisherAddrInfo(h host.Host) peer.AddrInfo {
	return peer.AddrInfo{
		ID:    h.ID(),
		Addrs: h.Addrs(),
	}
}

func ConnectToPeer(ctx context.Context, h host.Host, pAddr peer.AddrInfo) error {
	log.Printf("Node %s connecting to peer %s...", h.ID(), pAddr.ID)
	err := h.Connect(ctx, pAddr)
	if err != nil {
		log.Printf("Connection to peer failed: %v", err)
		return err
	} else {
		log.Printf("Connection to peer successful.")
		return nil
	}
}
