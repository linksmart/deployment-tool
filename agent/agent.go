package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"code.linksmart.eu/dt/deployment-tool/model"
	"github.com/satori/go.uuid"
)

type agent struct {
	sync.Mutex
	*model.Target
	configPath string

	pipe model.Pipe
}

func newAgent(pipe model.Pipe) *agent {
	a := &agent{
		pipe:       pipe,
		configPath: "config.json",
	}
	a.loadConf()

	log.Println("TargetID", a.ID)
	log.Println("CurrentTask", a.CurrentTask)
	log.Println("CurrentTaskStatus", a.CurrentTaskStatus)

	return a
}

func (a *agent) loadConf() {
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		log.Println("Configuration file not found.")
		a.ID = uuid.NewV4().String()
		log.Println("Generated target ID:", a.ID)

		a.saveConfig()
		return
	}

	b, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(b, a)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Loaded config file:", a.configPath)

}

func (a *agent) startTaskProcessor() {
	log.Println("Listenning for tasks...")

TASKLOOP:
	for task := range a.pipe.TaskCh {
		//log.Printf("taskProcessor: %+v", task)

		// TODO subscribe to next versions
		// For now, drop existing tasks
		for i := len(a.TaskHistory) - 1; i >= 0; i-- {
			if a.TaskHistory[i] == task.ID {
				log.Println("Existing task. Dropping it.")
				continue TASKLOOP
			}
		}

		a.TaskHistory = append(a.TaskHistory, task.ID)
		// send acknowledgement
		a.sendResponse(&model.BatchResponse{ResponseType: model.ResponseACK, TaskID: task.ID, TargetID: a.ID})

		a.storeArtifacts(task.Artifacts)

		// execute and collect results
		a.responseBatchCollector(task, time.Duration(3)*time.Second, a.pipe.ResponseCh)
	}

}

func (a *agent) saveConfig() {
	a.Lock()
	defer a.Unlock()

	b, err := json.MarshalIndent(a, "", "\t")
	if err != nil {
		log.Println(err)
		return
	}
	err = ioutil.WriteFile(a.configPath, b, 0600)
	if err != nil {
		log.Println("ERROR:", err)
		return
	}
	log.Println("Saved configuration:", a.configPath)
}

func (a *agent) sendResponse(resp *model.BatchResponse) {
	// send to channel
	a.pipe.ResponseCh <- *resp
	// update the status
	a.CurrentTask = resp.TaskID
	a.CurrentTaskStatus = resp.ResponseType
	a.saveConfig()
}

func (a *agent) close() {
	a.saveConfig()
}