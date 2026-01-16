# Harvester

## 🚀 Getting Started

### 1. Extract Dataset

The project requires a specific internal transaction dataset from XBlock-ETH project. Download 23250000to23499999_InternalTransaction.zip at https://xblock.pro/xblock-eth.html and Extract the zip file:

```bash
mkdir dataset
unzip dataset/23250000to23499999_InternalTransaction.zip -d dataset/

```

### 2. Build the Project

Ensure you have the **Go (Golang)** environment installed. Compile the core components using the Makefile:

```bash
mkdir bin
make all

```

### 3. Select Binary for Your Platform

After compilation, navigate to the `bin/` directory. You will find several executable binaries. Choose the one that matches your system architecture (e.g., `blockcooker_linux_amd64` or `blockcooker_darwin_arm64`).

### 4. Configuration

You need to configure path settings in the four JSON configuration files. Edit the following fields in each file:

* `storage_root_path`: Path for database storage.
* `input_dataset_path`: Path to the input dataset.
* `output_dataset_path`: Path for experimental results output.

**Configuration files to edit:**

* `config_generate_init_db.json`
* `config_tpe.json`
* `config_classic.json`
* `config_origin.json`

---

## 🧪 Running Experiments

### Step 1: Initialize the Database

Build the initial database containing **350 million accounts**:

```bash
./bin/blockcooker_YOUR_ARCH -c config_generate_init_db.json

```

### Step 2: Start the TPE Server

In a separate terminal, navigate to the `tpe_python` directory and start the parameter tuning server:

```bash
cd tpe_python
python tpe_server.py

```

### Step 3: Execute Comparative Experiments

Run the following commands to perform the experimental evaluations:

1. **Harvester-TPE Experiment** (Advanced tuning):
```bash
./bin/blockcooker_YOUR_ARCH -c config_tpe.json

```


2. **Harvester-Basic Experiment** (Standard version):
```bash
./bin/blockcooker_YOUR_ARCH -c config_classic.json

```


3. **Native Go-Ethereum Experiment** (Baseline):
```bash
./bin/blockcooker_YOUR_ARCH -c config_origin.json

```
