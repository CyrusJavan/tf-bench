# tf-bench
Benchmark terraform refresh performance of the resources in your workspace and generate a report.
![tf-bench-short](https://user-images.githubusercontent.com/8885202/127076367-1a3bd2f7-2dc5-4c4a-8005-966b342bcd25.gif)

## Example generated report
```
tf-bench (build time: 2021-07-25T18:05:55-07:00) Report 2021-07-25T18:05:31.57763-07:00
iterations per measurement: 3
terraform version: v1.0.0
provider versions:
registry.terraform.io/hashicorp/azurerm=2.66.0
registry.terraform.io/hashicorp/random=3.1.0
registry.terraform.io/aviatrixsystems/aviatrix=2.18.2
registry.terraform.io/hashicorp/aws=3.15.0

Refresh Time for Whole Workspace: 8.071s
+------------------------------+-------+---------------------------+---------------+---------+---------+--------+
| Resource Type                | Count | Average Time Per Resource | Average*Count | Minimum | Maximum | StdDev |
+------------------------------+-------+---------------------------+---------------+---------+---------+--------+
| aviatrix_vpc                 |     4 |                    3.082s |        12.33s |  2.474s |  3.782s |  438ms |
| aviatrix_controller_config   |     1 |                    3.745s |        3.745s |  3.186s |  4.066s |  397ms |
| aviatrix_gateway             |     1 |                    1.974s |        1.974s |  1.798s |  2.189s |  162ms |
| aviatrix_transit_gateway     |     1 |                     812ms |         812ms |   580ms |  1.067s |  199ms |
| aviatrix_device_registration |     1 |                     578ms |         578ms |   188ms |   812ms |  277ms |
| aviatrix_spoke_gateway       |     1 |                     163ms |         163ms |   161ms |   164ms |    2ms |
+------------------------------+-------+---------------------------+---------------+---------+---------+--------+
+------------------------------+----------------------------------------------------+----------------------------------------------------+
| Resource Type                | Fastest                                            | Slowest                                            |
+------------------------------+----------------------------------------------------+----------------------------------------------------+
| aviatrix_vpc                 | aviatrix_vpc.test_transit_vpc                      | aviatrix_vpc.vpc_a                                 |
| aviatrix_controller_config   | aviatrix_controller_config.test                    | aviatrix_controller_config.test                    |
| aviatrix_gateway             | aviatrix_gateway.test                              | aviatrix_gateway.test                              |
| aviatrix_transit_gateway     | aviatrix_transit_gateway.test_transit              | aviatrix_transit_gateway.test_transit              |
| aviatrix_device_registration | aviatrix_device_registration.device_registration_1 | aviatrix_device_registration.device_registration_1 |
| aviatrix_spoke_gateway       | aviatrix_spoke_gateway.gateway_a                   | aviatrix_spoke_gateway.gateway_a                   |
+------------------------------+----------------------------------------------------+----------------------------------------------------+
```

## Usage
Either clone this repository and `go build`, or download the latest release from the release page here on GitHub.
Then navigate to your terraform workspace and run:
```shell
tf-bench
```
The report will be written to a file as well as output to the console.

To see the possible options, run:
```shell
tf-bench --help
```
