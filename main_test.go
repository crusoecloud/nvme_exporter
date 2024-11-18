package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"reflect"
	"testing"
)

func TestNewNvmeCollector(t *testing.T) {
	temperatureScale := "celsius"
	collector := newNvmeCollector(&temperatureScale)

	if collector == nil {
		t.Fatalf("Expected newNvmeCollector to return a non-nil value")
	}
}

func TestNvmeCollector_Describe(t *testing.T) {
	temperatureScale := "celsius"
	collector := newNvmeCollector(&temperatureScale).(*nvmeCollector)

	ch := make(chan *prometheus.Desc)
	go func() {
		collector.Describe(ch)
		close(ch)
	}()

	for desc := range ch {
		if desc == nil {
			t.Errorf("Expected non-nil description")
		}
	}
}

/* TODO: work out how to test metrics, given the internals are hidden
func TestMakeMetric(t *testing.T) {
	temperatureScale := "celsius"
	collector := newNvmeCollector(&temperatureScale).(*nvmeCollector)
	desc := collector.nvmeTemperature
	metric := collector.makeMetric(desc, prometheus.GaugeValue, "250", "temperature", "/dev/nvme4n1")
	if metric == nil {
		t.Errorf("Expected non-nil metric")
	}
	if metric.val!= 250-273 {
		t.Errorf("Expected %dC, got %d", 250-273, metric)
	}
}
*/

func TestGetDeviceListV1(t *testing.T) {
	/*
		Modern versions of nvme-cli use 64bit ints for sizes, but have a new JSON format
	*/
	expectedOldDevices := []string{"/dev/nvme0n1"}
	oldDevicesJson := `{
      "Devices":[
			{
		  "NameSpace": 1,
		  "DevicePath": "/dev/nvme0n1",
		  "Firmware": "XXXXXXXX",
		  "ModelNumber": "XXXXXXX",
		  "SerialNumber": "XXXXXXX",
		  "UsedBytes": -2147483648,
		  "MaximumLBA": 1875385008,
		  "PhysicalSize": -2147483648,
		  "SectorSize": 512
		}
      ]
	}`
	if oldDevices := getDeviceList(oldDevicesJson); !reflect.DeepEqual(oldDevices, expectedOldDevices) {
		t.Errorf("Expected old format %s, got %s", expectedOldDevices, oldDevices)
	}
}
func TestGetDeviceListV2(t *testing.T) {
	expectedNewDevices := []string{"/dev/nvme2n1"}
	newDevicesJson := `{
      "Devices":[
		{
		  "HostNQN": "nqn.2014-08.org.nvmexpress:uuid:XXXXXXX",
		  "HostID": "XXXXXXX",
		  "Subsystems": [
		    {
		      "Subsystem": "nvme-subsys0",
		      "SubsystemNQN": "nqn.2016-08.com.micron:nvme:nvm-subsystem-sn-XXXXX",
		      "Controllers": [
		        {
		          "Controller": "nvme2",
		          "Cntlid": "0",
		          "SerialNumber": "XXXXXX",
		          "ModelNumber": "XXXXX",
		          "Firmware": "XXXXX",
		          "Transport": "pcie",
		          "Address": "0000:02:00.0",
		          "Slot": "9",
		          "Namespaces": [
		            {
		              "NameSpace": "nvme2n1",
		              "Generic": "ng2n1",
		              "NSID": 1,
		              "UsedBytes": 2097152,
		              "MaximumLBA": 25004872368,
		              "PhysicalSize": 12802494652416,
		              "SectorSize": 512
		            }
		          ],
		          "Paths": []
		        }
		      ],
		      "Namespaces": []
		    }
		  ]
		}
	  ]
	}`
	if newDevices := getDeviceList(newDevicesJson); !reflect.DeepEqual(newDevices, expectedNewDevices) {
		t.Errorf("Expected new format %s, got %s", expectedNewDevices, newDevices)
	}
}
func TestGetDeviceListV3(t *testing.T) {
	expectedDevices := []string{"/dev/nvme4n1"}
	devicesJson := `{
      "Devices":[
		{
		  "HostNQN": "nqn.2014-08.org.nvmexpress:uuid:XXXXXXX",
		  "HostID": "XXXXXXX",
		  "Subsystems": [
		    {
		      "Subsystem": "nvme-subsys0",
		      "SubsystemNQN": "nqn.2016-08.com.micron:nvme:nvm-subsystem-sn-XXXXX",
		      "Namespaces": [
		        {
		          "NameSpace": "nvme4n1",
		          "Generic": "ng4n1",
		          "NSID": 1,
		          "UsedBytes": 2097152,
		          "MaximumLBA": 25004872368,
		          "PhysicalSize": 12802494652416,
		          "SectorSize": 512
		        }
		      ]
		    }
		  ]
		}
	  ]
	}`
	if devices := getDeviceList(devicesJson); !reflect.DeepEqual(devices, expectedDevices) {
		t.Errorf("Expected new format %s, got %s", expectedDevices, devices)
	}
}
