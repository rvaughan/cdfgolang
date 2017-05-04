package main

import (
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io/ioutil"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "time"

    log "github.com/Sirupsen/logrus"
)


type ConfigObjectType struct {
}


func registerSIGINTHandler(cleanup chan bool) {
    // Register for SIGINT.
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

    // Start a goroutine that will unmount when the signal is received.
    go func() {
        for {
            s := <-signalChan
            log.WithFields(log.Fields{
                    "class": "Main",
                    "function": "registerSIGINTHandler",
                    "signal": s}).Warning("Shutting down.")

            cleanup <- true
        }
    }()
}


func read_config(filename string) (cfg ConfigObjectType, err error) {
    config_file_present := false
    for (!config_file_present) {
        if _, err := os.Stat(filename); os.IsNotExist(err) {
            log.WithFields(log.Fields{
                    "class": "Main",
                    "function": "read_config"}).Warning("Configuration file not present. Waiting for it to be created.")

            time.Sleep(5 * time.Second)
        } else {
            config_file_present = true
        }
    }

    config_data, e := ioutil.ReadFile(filename)
    if e != nil {
        err = e
        return
    }

    err = json.Unmarshal(config_data, &cfg)

    return
}


type scanCallback func(path string)


func monitor_config_changes(filename string, cb scanCallback) {
    startTime  := time.Now()

    config_dir := filepath.Dir(filename)
    config_filename := filepath.Base(filename)

    for {
        // log.Println("Checking to see if the configuration file has been updated.", filename, config_dir, config_filename)

        filepath.Walk(config_dir, func(path string, info os.FileInfo, err error) error {
            if path == ".git" {
                return filepath.SkipDir
            }

            // ignore hidden files
            if filepath.Base(path)[0] == '.' {
                return nil
            }

            // log.Println(filepath.Base(path), config_filename)

            if filepath.Base(path) == config_filename && info.ModTime().After(startTime) {
                cb(filename)

                startTime = time.Now()
                return errors.New("done")
            }

            return nil
        })

        // TODO: Probably should make this some kind of configurable item later.
        time.Sleep(10 * time.Second)
    }
}


func main() {
    config_filename := flag.String("config", "config.json", "The name of the configuration file to use.")

    flag.Parse()

    log.SetLevel(log.DebugLevel)

    config, err := read_config(*config_filename)
    if err == nil {
        cleanup_chan := make(chan bool, 1)
        registerSIGINTHandler(cleanup_chan)

        version_string := fmt.Sprintf("%s v%s", "App Name", "0.0.0")
        log.Info(version_string)

        log.Info(config)

        // Wait for the app to receive a shutdown request...
        <- cleanup_chan
    } else {
            log.WithFields(log.Fields{
                    "class": "Main",
                    "function": "main",
                    "error": err}).Error("Something is wrong with the configuration file. Unable to start.")
    }
}
