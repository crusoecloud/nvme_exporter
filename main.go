package main

// Export nvme smart-log metrics in prometheus format

import (
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tidwall/gjson"
	"log"
	"net/http"
	"os/exec"
	"os/user"
	"strings"
)

var (
	labels         = []string{"device"}
	maxTempSensors = 8 // as per NVMe spec
)

// NVMe spec says there are 0 to 8 temperature sensors
type nvmeCollector struct {
	nvmeCriticalWarning                    *prometheus.Desc
	nvmeAvailableSpare                     *prometheus.Desc
	nvmeTempThreshold                      *prometheus.Desc
	nvmeReliabilityDegraded                *prometheus.Desc
	nvmeRO                                 *prometheus.Desc
	nvmeVMBUFailed                         *prometheus.Desc
	nvmePMRRO                              *prometheus.Desc
	nvmeTemperature                        *prometheus.Desc
	nvmeAvailSpare                         *prometheus.Desc
	nvmeSpareThresh                        *prometheus.Desc
	nvmePercentUsed                        *prometheus.Desc
	nvmeEnduranceGrpCriticalWarningSummary *prometheus.Desc
	nvmeDataUnitsRead                      *prometheus.Desc
	nvmeDataUnitsWritten                   *prometheus.Desc
	nvmeHostReadCommands                   *prometheus.Desc
	nvmeHostWriteCommands                  *prometheus.Desc
	nvmeControllerBusyTime                 *prometheus.Desc
	nvmePowerCycles                        *prometheus.Desc
	nvmePowerOnHours                       *prometheus.Desc
	nvmeUnsafeShutdowns                    *prometheus.Desc
	nvmeMediaErrors                        *prometheus.Desc
	nvmeNumErrLogEntries                   *prometheus.Desc
	nvmeWarningTempTime                    *prometheus.Desc
	nvmeCriticalCompTime                   *prometheus.Desc
	nvmeTemperatureSensors                 []*prometheus.Desc
	nvmeThmTemp1TransCount                 *prometheus.Desc
	nvmeThmTemp2TransCount                 *prometheus.Desc
	nvmeThmTemp1TotalTime                  *prometheus.Desc
	nvmeThmTemp2TotalTime                  *prometheus.Desc
	temperatureScale                       *string
}

// nvme smart-log field descriptions can be found on page 180 of:
// https://nvmexpress.org/wp-content/uploads/NVM-Express-Base-Specification-2_0-2021.06.02-Ratified-5.pdf

func newNvmeCollector(temperatureScale *string) prometheus.Collector {
	var sensorDescriptions []*prometheus.Desc
	for i := 1; i <= maxTempSensors; i++ {
		description := prometheus.NewDesc(
			fmt.Sprintf("nvme_temperature_sensor%d", i),
			fmt.Sprintf("Temperature reported by thermal sensor #%d in degrees %s", i, *temperatureScale),
			labels,
			nil,
		)
		sensorDescriptions = append(sensorDescriptions, description)
	}

	return &nvmeCollector{
		temperatureScale: temperatureScale,
		nvmeCriticalWarning: prometheus.NewDesc(
			"nvme_critical_warning",
			"Critical warnings for the state of the controller",
			labels,
			nil,
		),
		nvmeAvailableSpare: prometheus.NewDesc(
			"nvme_available_spare_critical",
			"Has the 'available_spare' value dropped below 'spare_thresh'",
			labels,
			nil,
		),
		nvmeTempThreshold: prometheus.NewDesc(
			"nvme_temp_threshold_exceeded",
			"Temperature has exceeded the safe threshold",
			labels,
			nil,
		),
		nvmeReliabilityDegraded: prometheus.NewDesc(
			"nvme_reliability_degraded",
			"Device has degraded reliability due to excessive media/internal errors",
			labels,
			nil,
		),
		nvmeRO: prometheus.NewDesc(
			"nvme_readonly",
			"NVMe device is currently read-only",
			labels,
			nil,
		),
		nvmeVMBUFailed: prometheus.NewDesc(
			"nvme_vmbu_failed",
			"The 'Volatile Memory Backup Device' has failed, if present",
			labels,
			nil,
		),
		nvmePMRRO: prometheus.NewDesc(
			"nvme_pmr_ro",
			"The Persistent Memory Region is currently read-only",
			labels,
			nil,
		),
		nvmeTemperature: prometheus.NewDesc(
			"nvme_temperature",
			fmt.Sprintf("Temperature in degrees %s", *temperatureScale),
			labels,
			nil,
		),
		nvmeAvailSpare: prometheus.NewDesc(
			"nvme_avail_spare",
			"Normalized percentage of remaining spare capacity available",
			labels,
			nil,
		),
		nvmeSpareThresh: prometheus.NewDesc(
			"nvme_spare_thresh",
			"Async event completion may occur when avail spare < threshold",
			labels,
			nil,
		),
		nvmePercentUsed: prometheus.NewDesc(
			"nvme_percent_used",
			"Vendor specific estimate of the percentage of life used",
			labels,
			nil,
		),
		nvmeEnduranceGrpCriticalWarningSummary: prometheus.NewDesc(
			"nvme_endurance_grp_critical_warning_summary",
			"Critical warnings for the state of endurance groups",
			labels,
			nil,
		),
		nvmeDataUnitsRead: prometheus.NewDesc(
			"nvme_data_units_read",
			"Number of 512 byte data units host has read",
			labels,
			nil,
		),
		nvmeDataUnitsWritten: prometheus.NewDesc(
			"nvme_data_units_written",
			"Number of 512 byte data units the host has written",
			labels,
			nil,
		),
		nvmeHostReadCommands: prometheus.NewDesc(
			"nvme_host_read_commands",
			"Number of read commands completed",
			labels,
			nil,
		),
		nvmeHostWriteCommands: prometheus.NewDesc(
			"nvme_host_write_commands",
			"Number of write commands completed",
			labels,
			nil,
		),
		nvmeControllerBusyTime: prometheus.NewDesc(
			"nvme_controller_busy_time",
			"Amount of time in minutes controller busy with IO commands",
			labels,
			nil,
		),
		nvmePowerCycles: prometheus.NewDesc(
			"nvme_power_cycles",
			"Number of power cycles",
			labels,
			nil,
		),
		nvmePowerOnHours: prometheus.NewDesc(
			"nvme_power_on_hours",
			"Number of power on hours",
			labels,
			nil,
		),
		nvmeUnsafeShutdowns: prometheus.NewDesc(
			"nvme_unsafe_shutdowns",
			"Number of unsafe shutdowns",
			labels,
			nil,
		),
		nvmeMediaErrors: prometheus.NewDesc(
			"nvme_media_errors",
			"Number of unrecovered data integrity errors",
			labels,
			nil,
		),
		nvmeNumErrLogEntries: prometheus.NewDesc(
			"nvme_num_err_log_entries",
			"Lifetime number of error log entries",
			labels,
			nil,
		),
		nvmeWarningTempTime: prometheus.NewDesc(
			"nvme_warning_temp_time",
			"Amount of time in minutes temperature > warning threshold",
			labels,
			nil,
		),
		nvmeCriticalCompTime: prometheus.NewDesc(
			"nvme_critical_comp_time",
			"Amount of time in minutes temperature > critical threshold",
			labels,
			nil,
		),
		nvmeTemperatureSensors: sensorDescriptions,
		nvmeThmTemp1TransCount: prometheus.NewDesc(
			"nvme_thm_temp1_trans_count",
			"Number of times controller transitioned to lower power",
			labels,
			nil,
		),
		nvmeThmTemp2TransCount: prometheus.NewDesc(
			"nvme_thm_temp2_trans_count",
			"Number of times controller transitioned to lower power",
			labels,
			nil,
		),
		nvmeThmTemp1TotalTime: prometheus.NewDesc(
			"nvme_thm_temp1_trans_time",
			"Total number of seconds controller transitioned to lower power",
			labels,
			nil,
		),
		nvmeThmTemp2TotalTime: prometheus.NewDesc(
			"nvme_thm_temp2_trans_time",
			"Total number of seconds controller transitioned to lower power",
			labels,
			nil,
		),
	}
}

func (c *nvmeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.nvmeCriticalWarning
	ch <- c.nvmeAvailableSpare
	ch <- c.nvmeTempThreshold
	ch <- c.nvmeReliabilityDegraded
	ch <- c.nvmeRO
	ch <- c.nvmeVMBUFailed
	ch <- c.nvmePMRRO
	ch <- c.nvmeTemperature
	ch <- c.nvmeAvailSpare
	ch <- c.nvmeSpareThresh
	ch <- c.nvmePercentUsed
	ch <- c.nvmeEnduranceGrpCriticalWarningSummary
	ch <- c.nvmeDataUnitsRead
	ch <- c.nvmeDataUnitsWritten
	ch <- c.nvmeHostReadCommands
	ch <- c.nvmeHostWriteCommands
	ch <- c.nvmeControllerBusyTime
	ch <- c.nvmePowerCycles
	ch <- c.nvmePowerOnHours
	ch <- c.nvmeUnsafeShutdowns
	ch <- c.nvmeMediaErrors
	ch <- c.nvmeNumErrLogEntries
	ch <- c.nvmeWarningTempTime
	ch <- c.nvmeCriticalCompTime
	for i := 1; i <= maxTempSensors; i++ {
		ch <- c.nvmeTemperatureSensors[i-1]
	}
	ch <- c.nvmeThmTemp1TransCount
	ch <- c.nvmeThmTemp2TransCount
	ch <- c.nvmeThmTemp1TotalTime
	ch <- c.nvmeThmTemp2TotalTime
}

func (c *nvmeCollector) makeMetric(description *prometheus.Desc, valType prometheus.ValueType, result string, substring string, label string) prometheus.Metric {
	value := gjson.Get(result, substring).Float()
	if strings.Contains(substring, "temperature") {
		// Leave it alone, if it's in Kelvin, change if it's celsius or fahrenheit
		if *c.temperatureScale == "celsius" {
			value = value - 273
		}
		if *c.temperatureScale == "fahrenheit" {
			value = (value-273.15)*9/5 + 32
		}
	}
	return prometheus.MustNewConstMetric(description, valType, value, label)
}

func getDeviceList(nvmeListOutput string) []string {
	// There are three possible formats for
	if !gjson.Valid(string(nvmeListOutput)) {
		log.Fatalf("nvmeListOutput json is not valid\n%s", nvmeListOutput)
	}
	var deviceList []string
	nvmeJsonDeviceList := gjson.Get(string(nvmeListOutput), "Devices.#.DevicePath").Array()
	if len(nvmeJsonDeviceList) > 0 {
		log.Printf("with devicepath Devices: %v\n", nvmeJsonDeviceList)
		for _, devicePath := range nvmeJsonDeviceList {
			deviceList = append(deviceList, devicePath.String())
		}
		return deviceList
	}
	devices := gjson.Get(string(nvmeListOutput), "Devices.#.Subsystems.#.Namespaces.#.NameSpace")
	if len(devices.Array()) > 0 {
		for _, subsystems := range devices.Array() {
			for _, namespaces := range subsystems.Array() {
				for _, namespace := range namespaces.Array() {
					deviceList = append(deviceList, "/dev/"+namespace.String())
				}
			}
		}
	}
	if len(deviceList) > 0 {
		log.Printf("without controller Devices: %v\n", deviceList)
		return deviceList
	}
	// Some machines have multiple controllers, and 'nvme list -o json' adds them in, changing the output format
	devices = gjson.Get(string(nvmeListOutput), "Devices.#.Subsystems.#.Controllers.#.Namespaces.#.NameSpace")
	if len(devices.Array()) > 0 {
		for _, subsystems := range devices.Array() {
			for _, controllers := range subsystems.Array() {
				for _, namespaces := range controllers.Array() {
					for _, namespace := range namespaces.Array() {
						deviceList = append(deviceList, "/dev/"+namespace.String())
					}
				}
			}
		}
		log.Printf("with controller Devices: %v\n", deviceList)
		return deviceList
	} else {
		log.Fatal("No NVMe Devices found \n")
		return nil
	}
}

func (c *nvmeCollector) Collect(ch chan<- prometheus.Metric) {
	/*
		Old style JSON:
		# nvme smart-log -o json /dev/nvme0
		{
		  "critical_warning": 0,
		  "temperature": 310,
		  "avail_spare": 100,
		  "spare_thresh": 10,
		  "percent_used": 0,
		  "endurance_grp_critical_warning_summary": 0,
		  "data_units_read": 1765656.0,
		  "data_units_written": 4445011.0,
		  "host_read_commands": 195912399.0,
		  "host_write_commands": 433050333.0,
		  "controller_busy_time": 63.0,
		  "power_cycles": 27.0,
		  "power_on_hours": 6282.0,
		  "unsafe_shutdowns": 21.0,
		  "media_errors": 0.0,
		  "num_err_log_entries": 0.0,
		  "warning_temp_time": 0,
		  "critical_comp_time": 0,
		  "temperature_sensor_1": 318,
		  "temperature_sensor_2": 312,
		  "temperature_sensor_3": 310,
		  "thm_temp1_trans_count": 0,
		  "thm_temp2_trans_count": 0,
		  "thm_temp1_total_time": 0,
		  "thm_temp2_total_time": 0
		}

		New style JSON:
		#  nvme smart-log -o json /dev/nvme0
		{
		  "critical_warning": {
		    "value": 0,
		    "available_spare": 0,
		    "temp_threshold": 0,
		    "reliability_degraded": 0,
		    "ro": 0,
		    "vmbu_failed": 0,
		    "pmr_ro": 0
		  },
		  "temperature": 296,
		  "avail_spare": 100,
		  "spare_thresh": 10,
		  "percent_used": 0,
		  "endurance_grp_critical_warning_summary": 0,
		  "data_units_read": 4540,
		  "data_units_written": 16778,
		  "host_read_commands": 151174,
		  "host_write_commands": 99578,
		  "controller_busy_time": 1,
		  "power_cycles": 31,
		  "power_on_hours": 907,
		  "unsafe_shutdowns": 24,
		  "media_errors": 0,
		  "num_err_log_entries": 0,
		  "warning_temp_time": 0,
		  "critical_comp_time": 0,
		  "temperature_sensor_1": 302,
		  "temperature_sensor_2": 298,
		  "temperature_sensor_3": 296,
		  "thm_temp1_trans_count": 0,
		  "thm_temp2_trans_count": 0,
		  "thm_temp1_total_time": 0,
		  "thm_temp2_total_time": 0
		}

	*/
	nvmeListOutput, err := exec.Command("nvme", "list", "-o", "json").Output()
	if err != nil {
		log.Fatalf("Error running nvme command: %s\n", err)
	}
	nvmeDeviceList := getDeviceList(string(nvmeListOutput))

	for _, nvmeDevice := range nvmeDeviceList {
		nvmeSmartLog, err := exec.Command("nvme", "smart-log", nvmeDevice, "-o", "json").Output()
		nvmeSmartLogText := string(nvmeSmartLog)
		if err != nil {
			log.Fatalf("Error running nvme smart-log command for device %s: %s\n", nvmeDevice, err)
		}
		if !gjson.Valid(nvmeSmartLogText) {
			log.Fatalf("nvmeSmartLog json is not valid for device: %s: %s\n", nvmeDevice, err)
		}

		nvmeCriticalWarning := gjson.Get(nvmeSmartLogText, "critical_warning")
		if nvmeCriticalWarning.Type == gjson.JSON {
			// It's the new format, where 'critical' is a full JSON section; temperature_sensor_1 etc. push the last four down a row
			ch <- c.makeMetric(c.nvmeCriticalWarning, prometheus.GaugeValue, nvmeCriticalWarning.String(), "value", nvmeDevice)
			ch <- c.makeMetric(c.nvmeAvailableSpare, prometheus.GaugeValue, nvmeCriticalWarning.String(), "available_spare", nvmeDevice)
			ch <- c.makeMetric(c.nvmeTempThreshold, prometheus.GaugeValue, nvmeCriticalWarning.String(), "temp_threshold", nvmeDevice)
			ch <- c.makeMetric(c.nvmeReliabilityDegraded, prometheus.GaugeValue, nvmeCriticalWarning.String(), "reliability_degraded", nvmeDevice)
			ch <- c.makeMetric(c.nvmeRO, prometheus.GaugeValue, nvmeCriticalWarning.String(), "ro", nvmeDevice)
			ch <- c.makeMetric(c.nvmeVMBUFailed, prometheus.GaugeValue, nvmeCriticalWarning.String(), "vmbu_failed", nvmeDevice)
			ch <- c.makeMetric(c.nvmePMRRO, prometheus.GaugeValue, nvmeCriticalWarning.String(), "pmr_ro", nvmeDevice)

			for i := 1; i <= maxTempSensors; i++ {
				tempValue := gjson.Get(nvmeSmartLogText, fmt.Sprintf("temperature_sensor_%d", i))
				if !tempValue.Exists() {
					break
				}
				// ch <- prometheus.MustNewConstMetric(c.nvmeTemperatureSensors[i-1], prometheus.GaugeValue, tempValue.Float(), nvmeDevice)
				ch <- c.makeMetric(c.nvmeTemperatureSensors[i-1], prometheus.GaugeValue, nvmeSmartLogText, fmt.Sprintf("temperature_sensor_%d", i), nvmeDevice)
			}
		} else {
			ch <- c.makeMetric(c.nvmeCriticalWarning, prometheus.GaugeValue, nvmeSmartLogText, "critical_warning", nvmeDevice)
		}

		ch <- c.makeMetric(c.nvmeTemperature, prometheus.GaugeValue, nvmeSmartLogText, "temperature", nvmeDevice)
		ch <- c.makeMetric(c.nvmeAvailSpare, prometheus.GaugeValue, nvmeSmartLogText, "avail_spare", nvmeDevice)
		ch <- c.makeMetric(c.nvmeSpareThresh, prometheus.GaugeValue, nvmeSmartLogText, "spare_thresh", nvmeDevice)
		ch <- c.makeMetric(c.nvmePercentUsed, prometheus.GaugeValue, nvmeSmartLogText, "percent_used", nvmeDevice)
		ch <- c.makeMetric(c.nvmeEnduranceGrpCriticalWarningSummary, prometheus.GaugeValue, nvmeSmartLogText, "endurance_grp_critical_warning_summary", nvmeDevice)
		ch <- c.makeMetric(c.nvmeDataUnitsRead, prometheus.CounterValue, nvmeSmartLogText, "data_units_read", nvmeDevice)
		ch <- c.makeMetric(c.nvmeDataUnitsWritten, prometheus.CounterValue, nvmeSmartLogText, "data_units_written", nvmeDevice)
		ch <- c.makeMetric(c.nvmeHostReadCommands, prometheus.CounterValue, nvmeSmartLogText, "host_read_commands", nvmeDevice)
		ch <- c.makeMetric(c.nvmeHostWriteCommands, prometheus.CounterValue, nvmeSmartLogText, "host_write_commands", nvmeDevice)
		ch <- c.makeMetric(c.nvmeControllerBusyTime, prometheus.CounterValue, nvmeSmartLogText, "controller_busy_time", nvmeDevice)
		ch <- c.makeMetric(c.nvmePowerCycles, prometheus.CounterValue, nvmeSmartLogText, "power_cycles", nvmeDevice)
		ch <- c.makeMetric(c.nvmePowerOnHours, prometheus.CounterValue, nvmeSmartLogText, "power_on_hours", nvmeDevice)
		ch <- c.makeMetric(c.nvmeUnsafeShutdowns, prometheus.CounterValue, nvmeSmartLogText, "unsafe_shutdowns", nvmeDevice)
		ch <- c.makeMetric(c.nvmeMediaErrors, prometheus.CounterValue, nvmeSmartLogText, "media_errors", nvmeDevice)
		ch <- c.makeMetric(c.nvmeNumErrLogEntries, prometheus.CounterValue, nvmeSmartLogText, "num_err_log_entries", nvmeDevice)
		ch <- c.makeMetric(c.nvmeWarningTempTime, prometheus.CounterValue, nvmeSmartLogText, "warning_temp_time", nvmeDevice)
		ch <- c.makeMetric(c.nvmeCriticalCompTime, prometheus.CounterValue, nvmeSmartLogText, "critical_comp_time", nvmeDevice)
		ch <- c.makeMetric(c.nvmeThmTemp1TransCount, prometheus.CounterValue, nvmeSmartLogText, "thm_temp1_trans_count", nvmeDevice)
		ch <- c.makeMetric(c.nvmeThmTemp2TransCount, prometheus.CounterValue, nvmeSmartLogText, "thm_temp2_trans_count", nvmeDevice)
		ch <- c.makeMetric(c.nvmeThmTemp1TotalTime, prometheus.CounterValue, nvmeSmartLogText, "thm_temp3_total_time", nvmeDevice)
		ch <- c.makeMetric(c.nvmeThmTemp2TotalTime, prometheus.CounterValue, nvmeSmartLogText, "thm_temp1_total_time", nvmeDevice)
	}
}

func main() {
	port := flag.String("port", "9998", "port to listen on")
	temperatureScale := flag.String("temperature_scale", "celsius", "One of : [celsius | fahrenheit | kelvin]. NVMe standard recommens Kelvin.")
	flag.Parse()
	// check user
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("Error getting current user  %s\n", err)
	}
	if currentUser.Username != "root" {
		log.Fatalln("Error: you must be root to use nvme-cli")
	}
	// check for nvme-cli executable
	_, err = exec.LookPath("nvme")
	if err != nil {
		log.Fatalf("Cannot find nvme command in path: %s\n", err)
	}
	prometheus.MustRegister(newNvmeCollector(temperatureScale))
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
