package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	log "github.com/sirupsen/logrus"
)

func loadConfig(path string) Config {
	jsonObject, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	decoder := json.NewDecoder(jsonObject)
	_ = decoder.Decode(&config)
	_ = jsonObject.Close()
	return config
}

func deferInit() {
	flag.StringVar(&configPath, "config", "config.json", "/path/to/config.json. (Default: ./config.json)")
	flag.BoolVar(&prefetch, "prefetch", false, "Prefetch and convert image to webp")
	flag.IntVar(&jobs, "jobs", runtime.NumCPU(), "Prefetch thread, default is all.")
	flag.BoolVar(&dumpConfig, "dump-config", false, "Print sample config.json")
	flag.BoolVar(&dumpSystemd, "dump-systemd", false, "Print sample systemd service file.")
	flag.BoolVar(&verboseMode, "v", false, "Verbose, print out debug info.")
	flag.BoolVar(&showVersion, "V", false, "Show version information.")
	flag.Parse()
	// Logrus
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)
	Formatter := &log.TextFormatter{
		EnvironmentOverrideColors: true,
		FullTimestamp:             true,
		TimestampFormat:           "2006-01-02 15:04:05",
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			return fmt.Sprintf("[%s()]", f.Function), ""
		},
	}
	log.SetFormatter(Formatter)

	if verboseMode {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode is enable!")
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

func switchProxyMode() {
	// Check for remote address
	matched, _ := regexp.MatchString(`^https?://`, config.ImgPath)
	proxyMode = false
	if matched {
		proxyMode = true
	} else {
		_, err := os.Stat(config.ImgPath)
		if err != nil {
			log.Fatalf("Your image path %s is incorrect.Please check and confirm.", config.ImgPath)
		}
	}
}

func main() {
	// Our banner
	banner := fmt.Sprintf(`
▌ ▌   ▌  ▛▀▖ ▞▀▖                ▞▀▖
▌▖▌▞▀▖▛▀▖▙▄▘ ▚▄ ▞▀▖▙▀▖▌ ▌▞▀▖▙▀▖ ▌▄▖▞▀▖
▙▚▌▛▀ ▌ ▌▌   ▖ ▌▛▀ ▌  ▐▐ ▛▀ ▌   ▌ ▌▌ ▌
▘ ▘▝▀▘▀▀ ▘   ▝▀ ▝▀▘▘   ▘ ▝▀▘▘   ▝▀ ▝▀

Webp Server Go - v%s
Develop by WebP Server team. https://github.com/webp-sh`, version)

	deferInit()
	// process cli params
	if dumpConfig {
		fmt.Println(sampleConfig)
		os.Exit(0)
	}
	if dumpSystemd {
		fmt.Println(sampleSystemd)
		os.Exit(0)
	}
	if showVersion {
		fmt.Printf("\n %c[1;32m%s%c[0m\n\n", 0x1B, banner+"", 0x1B)
		os.Exit(0)
	}

	go autoUpdate()
	config = loadConfig(configPath)
	switchProxyMode()

	if prefetch {
		go prefetchImages(config.ImgPath, config.ExhaustPath, config.Quality)
	}

	app := fiber.New(fiber.Config{
		ServerHeader:          "Webp-Server-Go",
		DisableStartupMessage: true,
	})
	app.Use(logger.New())

    // ★新增: /healthz 路由
    app.Get("/healthz", func(c *fiber.Ctx) error {
        // 检查图片根目录是否可访问；你也可换成其他探针
        if _, err := os.Stat(config.ImgPath); err != nil {
            return c.Status(fiber.StatusServiceUnavailable).
                JSON(fiber.Map{"status": "unhealthy"})
        }
        return c.JSON(fiber.Map{"status": "ok"})
    })

	listenAddress := config.Host + ":" + config.Port
	app.Get("/*", convert)

	fmt.Printf("\n %c[1;32m%s%c[0m\n\n", 0x1B, banner, 0x1B)
	fmt.Println("Webp-Server-Go is Running on http://" + listenAddress)

	_ = app.Listen(listenAddress)

}
