# 项目名称，编译后生成的可执行文件名
PROJECT_NAME := blockcooker

# 源代码入口文件，通常是 main.go
SRC_DIR := .
MAIN_FILE := cmd/main.go

# 输出目录
BUILD_DIR := ./bin


# 默认编译所有常用版本：Linux/AMD64/386, Darwin/ARM64/AMD64, Windows/AMD64/386
all: build-linux-arm build-linux-x86 build-darwin-arm build-windows-x86 build-linux-386 build-darwin-x86 build-windows-386

# --- Linux 编译目标 ---

build-linux-arm:
	@echo "--- 正在编译 Linux/ARM64 可执行文件 ---"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(PROJECT_NAME)-linux-arm64 $(SRC_DIR)/$(MAIN_FILE)
	@echo "编译完成：$(BUILD_DIR)/$(PROJECT_NAME)-linux-arm64"

build-linux-x86:
	@echo "--- 正在编译 Linux/AMD64 (x86-64) 可执行文件 ---"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(PROJECT_NAME)-linux-amd64 $(SRC_DIR)/$(MAIN_FILE)
	@echo "编译完成：$(BUILD_DIR)/$(PROJECT_NAME)-linux-amd64"

build-linux-386:
	@echo "--- 正在编译 Linux/386 (x86 32位) 可执行文件 ---"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=386 go build -o $(BUILD_DIR)/$(PROJECT_NAME)-linux-386 $(SRC_DIR)/$(MAIN_FILE)
	@echo "编译完成：$(BUILD_DIR)/$(PROJECT_NAME)-linux-386"

# --- Darwin (macOS) 编译目标 ---

build-darwin-arm:
	@echo "--- 正在编译 Darwin/ARM64 (macOS Apple Silicon) 可执行文件 ---"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin-arm64 $(SRC_DIR)/$(MAIN_FILE)
	@echo "编译完成：$(BUILD_DIR)/$(PROJECT_NAME)-darwin-arm64"

build-darwin-x86:
	@echo "--- 正在编译 Darwin/AMD64 (macOS Intel) 可执行文件 ---"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin-amd64 $(SRC_DIR)/$(MAIN_FILE)
	@echo "编译完成：$(BUILD_DIR)/$(PROJECT_NAME)-darwin-amd64"

# --- Windows 编译目标 ---

build-windows-x86:
	@echo "--- 正在编译 Windows/AMD64 (x86-64) 可执行文件 (.exe) ---"
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(PROJECT_NAME)-windows-amd64.exe $(SRC_DIR)/$(MAIN_FILE)
	@echo "编译完成：$(BUILD_DIR)/$(PROJECT_NAME)-windows-amd64.exe"

build-windows-386:
	@echo "--- 正在编译 Windows/386 (x86 32位) 可执行文件 (.exe) ---"
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=386 go build -o $(BUILD_DIR)/$(PROJECT_NAME)-windows-386.exe $(SRC_DIR)/$(MAIN_FILE)
	@echo "编译完成：$(BUILD_DIR)/$(PROJECT_NAME)-windows-386.exe"

## 清理编译生成的文件
clean:
	@echo "--- 正在清理编译生成的文件 ---"
	@rm -rf $(BUILD_DIR)
	@echo "清理完成"
