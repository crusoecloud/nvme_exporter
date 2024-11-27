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
	expectedOldDevices := []nvmeNamespace{{
		devicePath:     "/dev/nvme0n1",
		nsController:   "nvme0",
		nsMaximumLBA:   -1,
		nsSectorSize:   -1,
		nsUsedBytes:    -1,
		nsPhysicalSize: -1,
	}}
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
		t.Errorf("Expected old format %v, got %v", expectedOldDevices, oldDevices)
	}
}
func TestGetDeviceListV2(t *testing.T) {
	expectedNewDevices := []nvmeNamespace{{
		devicePath:     "/dev/nvme2n1",
		nsController:   "nvme2",
		nsMaximumLBA:   25004872368,
		nsSectorSize:   512,
		nsUsedBytes:    2097152,
		nsPhysicalSize: 12802494652416,
	}}
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
		t.Errorf("Expected new format %v, got %v", expectedNewDevices, newDevices)
	}
}
func TestGetDeviceListV3(t *testing.T) {
	expectedDevices := []nvmeNamespace{{
		devicePath:     "/dev/nvme4n1",
		nsController:   "nvme4",
		nsMaximumLBA:   25004872368,
		nsSectorSize:   512,
		nsUsedBytes:    2097152,
		nsPhysicalSize: 12802494652416,
	}}
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
		t.Errorf("Expected new format %v, got %v", expectedDevices, devices)
	}
}

func TestGetDeviceListV4(t *testing.T) {
	expectedDevices := []nvmeNamespace{
		{
			devicePath:     "/dev/nvme9n1",
			nsController:   "nvme9",
			nsMaximumLBA:   268435456,
			nsSectorSize:   512,
			nsUsedBytes:    137438953472,
			nsPhysicalSize: 137438953472,
		},
		{
			devicePath:     "/dev/nvme3n1",
			nsController:   "nvme3",
			nsMaximumLBA:   1875385008,
			nsSectorSize:   512,
			nsUsedBytes:    7486193664,
			nsPhysicalSize: 960197124096,
		},
	}

	mixedDevicesJson := `{
  "Devices":[
    {
      "HostNQN":"2f91420a-922f-5e49-b86e-b6bdc2f62412",
      "HostID":"95a57b1a-be86-4528-b093-57c49ee379d6",
      "Subsystems":[
        {
          "Subsystem":"nvme-subsys9",
          "SubsystemNQN":"nqn.2016-01.com.lightbitslabs:uuid:696-aa11-4912-acf9-eb2cfcd",
          "Controllers":[
            {
              "Controller":"nvme0",
              "Cntlid":"566",
              "SerialNumber":"6ab81c478ac",
              "ModelNumber":"Lightbits LightOS",
              "Firmware":"3.6",
              "Transport":"tcp",
              "Address":"traddr=10.50.4.15,trsvcid=4421",
              "Slot":"",
              "Namespaces":[
              ],
              "Paths":[
                {
                  "Path":"nvme9c0n1",
                  "ANAState":"inaccessible"
                }
              ]
            },
            {
              "Controller":"nvme10",
              "Cntlid":"390",
              "SerialNumber":"69a8c48ac",
              "ModelNumber":"Lightbits LightOS",
              "Firmware":"3.6",
              "Transport":"tcp",
              "Address":"traddr=10.50.4.14,trsvcid=4421",
              "Slot":"",
              "Namespaces":[
              ],
              "Paths":[
              ]
            },
            {
              "Controller":"nvme11",
              "Cntlid":"3843",
              "SerialNumber":"69a8c48ac",
              "ModelNumber":"Lightbits LightOS",
              "Firmware":"3.6",
              "Transport":"tcp",
              "Address":"traddr=10.50.4.13,trsvcid=4420",
              "Slot":"",
              "Namespaces":[
              ],
              "Paths":[
                {
                  "Path":"nvme9c11n1",
                  "ANAState":"optimized"
                }
              ]
            },
            {
              "Controller":"nvme9",
              "Cntlid":"5126",
              "SerialNumber":"69a8c48ac",
              "ModelNumber":"Lightbits LightOS",
              "Firmware":"3.6",
              "Transport":"tcp",
              "Address":"traddr=10.50.4.11,trsvcid=4421",
              "Slot":"",
              "Namespaces":[
              ],
              "Paths":[
              ]
            }
          ],
          "Namespaces":[
            {
              "NameSpace":"nvme9n1",
              "Generic":"ng9n1",
              "NSID":29017,
              "UsedBytes":137438953472,
              "MaximumLBA":268435456,
              "PhysicalSize":137438953472,
              "SectorSize":512
            }
          ]
        },
        {
          "Subsystem":"nvme-subsys3",
          "SubsystemNQN":"nqn.2016-08.com.micron:nvme:nvm-subsystem-sn-2402473F6E8C",
          "Controllers":[
            {
              "Controller":"nvme3",
              "Cntlid":"0",
              "SerialNumber":"24023F6EC",
              "ModelNumber":"Micron_7450_MTF960TFR",
              "Firmware":"EMU20",
              "Transport":"pcie",
              "Address":"0000:4f:00.0",
              "Slot":"12",
              "Namespaces":[
                {
                  "NameSpace":"nvme3n1",
                  "Generic":"ng3n1",
                  "NSID":1,
                  "UsedBytes":7486193664,
                  "MaximumLBA":1875385008,
                  "PhysicalSize":960197124096,
                  "SectorSize":512
                }
              ],
              "Paths":[
              ]
            }
          ],
          "Namespaces":[
          ]
        }
      ]
    }
  ]
}
`
	if devices := getDeviceList(mixedDevicesJson); !reflect.DeepEqual(devices, expectedDevices) {
		t.Errorf("Expected new format %v, got %v", expectedDevices, devices)
	}

}
