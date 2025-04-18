package main

import (
	"embed"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/J-A-M-P-S/structs"

	"github.com/dominant-strategies/go-quai-stratum/api"
	"github.com/dominant-strategies/go-quai-stratum/proxy"
	"github.com/dominant-strategies/go-quai-stratum/storage"
	"github.com/dominant-strategies/go-quai-stratum/util"

	"github.com/dominant-strategies/go-quai/cmd/utils"
	"github.com/dominant-strategies/go-quai/common"
	"github.com/dominant-strategies/go-quai/log"
)

var cfg proxy.Config
var backend *storage.RedisClient

// MinerManager to track the GPU miner process
type MinerManager struct {
	cmd     *exec.Cmd
	running bool
}

// Global instance of MinerManager
var minerManager MinerManager

func startProxy() {
	s := proxy.NewProxy(&cfg, backend)
	s.Start()
}

func startApi() {
	settings := structs.Map(&cfg)
	s := api.NewApiServer(&cfg.Api, settings, backend)
	s.Start()
}

func readConfig(cfg *proxy.Config) {
	configPath := flag.String("config", "config/config.json", "Path to config file")
	zoneUrl := flag.String("zone", "", "Zone upstream port (overrides config)")

	stratumPort := flag.Int("stratum", -1, "Stratum listen port (overrides config)")

	gpuType := flag.String("gpuType", "", "Gpu type either (nvidia/amd)")

	// Flags for Seal Mining
	quaiCoinbase := flag.String("quaiCoinbase", "", "")
	qiCoinbase := flag.String("qiCoinbase", "", "")
	minerPreference := flag.Float64("minerPreference", 0.5, "")

	flag.Parse()

	log.Global.WithField(
		"path", *configPath,
	).Info("Loading config")

	// Read config file.
	configFile, err := os.Open(*configPath)
	if err != nil {
		log.Global.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&cfg); err != nil {
		log.Global.Fatal("Config error: ", err.Error())
	}

	if gpuType != nil && *gpuType != "" {
		cfg.Mining.Enabled = true
		cfg.Mining.GpuType = *gpuType
	}

	if quaiCoinbase != nil && *quaiCoinbase != "" {
		if !common.IsHexAddress(*quaiCoinbase) {
			log.Global.WithField("quaiCoinbase", *quaiCoinbase).Fatal("Invalid quaiCoinbase")
		}
		cfg.Proxy.QuaiCoinbase = common.HexToAddress(*quaiCoinbase, common.Location{0, 0})
	}

	if qiCoinbase != nil && *qiCoinbase != "" {
		if !common.IsHexAddress(*qiCoinbase) {
			log.Global.WithField("qiCoinbase", *qiCoinbase).Fatal("Invalid qiCoinbase")
		}
		cfg.Proxy.QiCoinbase = common.HexToAddress(*qiCoinbase, common.Location{0, 0})
	}

	if minerPreference != nil {
		if *minerPreference < 0 || *minerPreference > 1 {
			log.Global.WithField("minerPreference", *minerPreference).Fatal("Invalid minerPreference")
		}
		cfg.Proxy.MinerPreference = *minerPreference
	}

	// Perform custom overrides. Default means they weren't set on the command line.
	if zoneUrl != nil && *zoneUrl != "" {
		cfg.Upstream.Name = "cyprus1"
		cfg.Upstream.Url = *zoneUrl
	}
	if *stratumPort != -1 {
		cfg.Proxy.Stratum.Listen = "0.0.0.0:" + strconv.Itoa(*stratumPort)
	}
}

func returnPortHelper(locName string) string {
	var portStr string
	// Check if already a port number, otherwise look up by name.
	if _, err := strconv.Atoi(locName); err != nil {
		loc, err := util.LocationFromName(locName)
		if err != nil {
			log.Global.WithField("err", err).Warn("Unable to parse location")
		}
		portStr = strconv.Itoa(utils.GetWSPort(loc))
	}
	return portStr
}

func init() {
}

//go:embed gpu-miner/go-quai-stratum-miner-*
var gpuMiners embed.FS

func startGpuMiner(config proxy.Config) {

	// Delay for the proxy to fully initialize.
	time.Sleep(5 * time.Second)
	// Define the binary name based on GPU type
	binaryName := "go-quai-stratum-miner-" + config.Mining.GpuType
	embeddedPath := "gpu-miner/" + binaryName

	// Read the embedded binary
	binaryData, err := gpuMiners.ReadFile(embeddedPath)
	if err != nil {
		log.Global.Errorf("Failed to read embedded binary %s: %v", binaryName, err)
		return
	}

	// Create a temporary file or use a fixed path for the binary
	binaryPath := filepath.Join("gpu-miner", binaryName)
	if err := os.MkdirAll("gpu-miner", 0755); err != nil {
		log.Global.Errorf("Failed to create gpu-miner directory: %v", err)
		return
	}

	// Write the binary to disk
	if err := os.WriteFile(binaryPath, binaryData, 0755); err != nil {
		log.Global.Errorf("Failed to write binary %s: %v", binaryName, err)
		return
	}

	// Ensure the binary is executable (important for Linux/macOS)
	if err := os.Chmod(binaryPath, 0755); err != nil {
		log.Global.Errorf("Failed to set executable permissions for %s: %v", binaryName, err)
		return
	}

	// Initialize the command
	var gpuFlag string
	if config.Mining.GpuType == "nvidia" {
		gpuFlag = "-U"
	} else if config.Mining.GpuType == "amd" {
		gpuFlag = "-G"
	}

	minerManager.cmd = exec.Command(binaryPath, gpuFlag, "-P", "stratum://"+config.Proxy.Stratum.Listen)
	if err := minerManager.cmd.Start(); err != nil {
		log.Global.Warnf("Failed to start GPU miner: %v", err)
		return
	}
	minerManager.running = true
	log.Global.Info("GPU miner running...")
}

// StopAllGpuMiners kills all go-quai-stratum-miner processes
func StopAllGpuMiners() {
	var cmd *exec.Cmd
	// Use platform-specific command to kill all go-quai-stratum-miner processes
	switch runtime.GOOS {
	case "linux", "darwin":
		// Use pkill to kill all processes matching the name
		cmd = exec.Command("pkill", "-f", "go-quai-stratum-miner")
	}

	// Execute the kill command
	if err := cmd.Run(); err != nil {
		log.Global.Warnf("Failed to kill all GPU miners: %v", err)
	} else {
		log.Global.Info("All GPU miners stopped")
	}

	// Reset the minerManager state
	minerManager.running = false
	minerManager.cmd = nil
}

func main() {
	readConfig(&cfg)

	if cfg.Threads > 0 {
		runtime.GOMAXPROCS(cfg.Threads)
		log.Global.WithField(
			"threads", cfg.Threads,
		).Debug("Threads running")
	}

	if cfg.Proxy.Enabled {
		go startProxy()
	}
	if cfg.Api.Enabled {
		go startApi()
	}

	if cfg.Mining.Enabled {
		go startGpuMiner(cfg)
	}

	// Set up signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received
	<-quit
	log.Global.Info("Received shutdown signal, stopping...")

	if cfg.Mining.Enabled {
		// kill the gpu miner on stop
		StopAllGpuMiners()
	}

	log.Global.Info("Stratum stopped")
}
