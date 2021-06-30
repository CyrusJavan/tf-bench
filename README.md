# tf-bench
Benchmark terraform refresh performance of the resources in your workspace and generate a report.

## Example generated report
```
tf-bench Report 2021-06-30T15:55:21-07:00
iterations per measurement: 1
terraform version: v0.12.5
provider versions:
aws=3.37.0
azurerm=2.56.0
google=3.65.0
random=3.1.0
aviatrix=2.18.2

Refresh Time for Whole Workspace: 13.514001184s
+------------------------------------------------+
| Individual Refresh Statistics                  |
+------------------------+-------+---------------+
| TYPE                   | COUNT |  REFRESH TIME |
+------------------------+-------+---------------+
| azurerm_resource_group |     1 | 11.450071408s |
| aws_s3_bucket          |     1 |  8.310989713s |
| aviatrix_vpc           |     1 |  5.119822706s |
| google_service_account |     1 |  2.889675972s |
| random_id              |    10 |  2.305856125s |
+------------------------+-------+---------------+
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
