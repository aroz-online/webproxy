package main

import (
	"aroz-online/webproxy/mod/aroz"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/smartystreets/cproxy"
)

/*
	Basic Web Proxy Module
	Allowing anyone to proxy through this machine to serf the web

*/

type config struct {
	Proxyport int      `json:"proxyport"`
	Defaulton bool     `json:"defaulton"`
	Whitelist []string `json:"whitelist"`
	Blacklist []string `json:"blacklist"`
}

type DefaultFilter struct{}

var (
	arozHandler aroz.ArozHandler
	proxyOnline = false
	proxyConfig config
)

//Kill signal handler. Do something before the system the core terminate.
func SetupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("\r- Shutting down webproxy service.")
		//Do other things like close database or opened files

		os.Exit(0)
	}()
}

func main() {
	//Start the aoModule pipeline (which will parse the flags as well). Pass in the module launch information
	arozHandler := aroz.HandleFlagParse(aroz.ServiceInfo{
		Name:     "WebProxy",
		Desc:     "A mini web proxy subservices for proxying website with your ArozOS Host",
		Group:    "Internet",
		IconPath: "webproxy/img/icon.png",
		Version:  "0.1",
		//You can define any path before the actualy html file. This directory (in this case demo/ ) will be the reverse proxy endpoint for this module
		StartDir:    "webproxy/index.html",
		SupportFW:   true,
		LaunchFWDir: "webproxy/index.html",
		InitFWSize:  []int{480, 640},
	})

	//Print startup and license information
	log.Println("ArOZ WebProxy - Powered by cproxy library")
	log.Println(`Copyright 2020-2021 tobychui
	Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
	The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.`)

	//Register the standard web services urls
	fs := http.FileServer(http.Dir("./ui"))
	http.Handle("/", fs)
	http.HandleFunc("/toggle", handleProxyToggle)
	http.HandleFunc("/status", showProxyStatus)
	http.HandleFunc("/info", handleProxyInfo)

	SetupCloseHandler()

	//Read settings from config.json
	//Check if the file exists in the cwd. If not, following aroz online convension
	filepath := "./config.json"
	if !fileExists(filepath) {
		//Generate a new config file in this folder
		ioutil.WriteFile("config.json", []byte(`{
			"proxyport": 8081,
			"defaulton":true,
			"whitelist":[],
			"blacklist":[]
		}`), 0777)
	}

	//Parse setting
	settingFileContent, err := ioutil.ReadFile(filepath)
	if err != nil {
		panic(err)
	}
	proxySetting := new(config)
	err = json.Unmarshal(settingFileContent, &proxySetting)
	if err != nil {
		panic(err)
	}

	//Update default proxy status
	proxyConfig = *proxySetting
	proxyOnline = proxyConfig.Defaulton

	//Create the webproxy service in go routine
	go func(proxySetting config) {
		filter := &DefaultFilter{}
		handler := cproxy.Configure(cproxy.WithFilter(filter))
		log.Println("WebProxy listening on:", "*:"+IntToString(proxySetting.Proxyport))
		http.ListenAndServe(":"+IntToString(proxySetting.Proxyport), handler)
	}(*proxySetting)

	//Start UI elements in main thread
	log.Println("Web Proxy Service UI Started: " + arozHandler.Port)
	err = http.ListenAndServe(arozHandler.Port, nil)
	if err != nil {
		log.Fatal(err)
	}

}

func (it *DefaultFilter) IsAuthorized(r *http.Request) bool {
	if !proxyOnline {
		return false
	}
	if len(proxyConfig.Whitelist) > 0 {
		//Match whitelist. Check if this is in the whitelist allowed urls
		for _, whitelistURL := range proxyConfig.Whitelist {
			if strings.Contains(r.URL.String(), whitelistURL) {
				return true
			}
		}
		log.Println("Refusing connection to: ", r.URL)
		return false
	} else if len(proxyConfig.Blacklist) > 0 {
		//Match blacklist
		for _, blacklistURL := range proxyConfig.Blacklist {
			if strings.Contains(r.URL.String(), blacklistURL) {
				log.Println("Refusing connection to: ", r.URL)
				return false
			}
		}
		return true
	} else {
		return true
	}
}

func handleProxyToggle(w http.ResponseWriter, r *http.Request) {
	status, err := mv(r, "opr", false)
	if err != nil {
		sendErrorResponse(w, "opr not defined")
		return
	}
	if status == "on" {
		proxyOnline = true
		sendOK(w)
	} else if status == "off" {
		proxyOnline = false
		sendOK(w)
	} else {
		sendErrorResponse(w, "Invalid opr given")
		return
	}
}

func handleProxyInfo(w http.ResponseWriter, r *http.Request) {
	jsonString, _ := json.Marshal(proxyConfig)
	sendJSONResponse(w, string(jsonString))
}

func showProxyStatus(w http.ResponseWriter, r *http.Request) {
	js, _ := json.Marshal(proxyOnline)
	sendJSONResponse(w, string(js))
}
