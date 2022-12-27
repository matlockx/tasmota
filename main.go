package main

import (
	"encoding/json"
	"flag"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/influxdata/influxdb1-client" // this is important because of the bug in go mod
	influx "github.com/influxdata/influxdb1-client/v2"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mqttAddress := flag.String("mqtt-address", "192.168.2.3:1883", "mqtt broker")
	influxAddress := flag.String("influx-address", "http://192.168.2.3:8086", "influx broker")
	logrus.Infof("using mqtt %s", *mqttAddress)
	logrus.Infof("using influx %s", *influxAddress)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	influxClient, _ := influx.NewHTTPClient(influx.HTTPConfig{
		Addr:     *influxAddress,
		Username: "admin",
		Password: "admin",
	})

	clientOptions := mqtt.NewClientOptions().AddBroker(*mqttAddress).SetClientID("tasmota")

	mqttClient := mqtt.NewClient(clientOptions)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		logrus.Panic(token.Error())
	}

	if token := mqttClient.Subscribe("tele/#", 0, onMessage(influxClient)); token.Wait() && token.Error() != nil {
		logrus.Panic(token.Error())
	}

	defer mqttClient.Disconnect(1000)
	<-c
}

func onMessage(influxClient influx.Client) func(client mqtt.Client, message mqtt.Message) {
	return func(client mqtt.Client, message mqtt.Message) {
		switch message.Topic() {
		case "tele/tasmota_FA2642/SENSOR":
			logrus.Infof("Topic: %s Payload: %s", message.Topic(), message.Payload())
			sendSensorPayload("Power", message, influxClient)
		case "tele/tasmota_5BEF46/SENSOR":
			logrus.Infof("Topic: %s Payload: %s", message.Topic(), message.Payload())
			sendSensorPayload("Heating", message, influxClient)
		}
	}
}

func sendSensorPayload(counter string, message mqtt.Message, writeApi influx.Client) {
	var sensor SensorPayload
	err := json.Unmarshal(message.Payload(), &sensor)
	if err != nil {
		logrus.Errorf("parse sensor payload: %s", err)
		return
	}
	fields := map[string]any{

		"counter": counter,
		"totalIn": sensor.MT681.TotalIn,
		"current": sensor.MT681.PowerCur,
		"meterId": sensor.MT681.MeterId,
	}
	p, _ := influx.NewPoint("MT681", map[string]string{"counter": counter}, fields, time.Now())
	batch, _ := influx.NewBatchPoints(influx.BatchPointsConfig{Database: "db0"})
	batch.AddPoint(p)
	err = writeApi.Write(batch)
	if err != nil {
		logrus.Errorf("error writing point: %s", err)
	}
}

type SensorPayload struct {
	Time  string `json:"Time"`
	MT681 struct {
		TotalIn  float64 `json:"Total_in"`
		PowerCur int     `json:"Power_cur"`
		PowerP1  int     `json:"Power_p1"`
		PowerP2  int     `json:"Power_p2"`
		PowerP3  int     `json:"Power_p3"`
		TotalOut float64 `json:"Total_out"`
		MeterId  string  `json:"Meter_id"`
	} `json:"MT681"`
}

type StatePayload struct {
	Time      string `json:"Time"`
	Uptime    string `json:"Uptime"`
	UptimeSec int    `json:"UptimeSec"`
	Heap      int    `json:"Heap"`
	SleepMode string `json:"SleepMode"`
	Sleep     int    `json:"Sleep"`
	LoadAvg   int    `json:"LoadAvg"`
	MqttCount int    `json:"MqttCount"`
	POWER     string `json:"POWER"`
	Wifi      struct {
		AP        int    `json:"AP"`
		SSId      string `json:"SSId"`
		BSSId     string `json:"BSSId"`
		Channel   int    `json:"Channel"`
		Mode      string `json:"Mode"`
		RSSI      int    `json:"RSSI"`
		Signal    int    `json:"Signal"`
		LinkCount int    `json:"LinkCount"`
		Downtime  string `json:"Downtime"`
	} `json:"Wifi"`
}
