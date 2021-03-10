# Lambda-CI

## Description

Lambda-CI is a small tool that searches recursively for Lambda functions to build and deploy.
It looks for `.function.yaml` files in all subdirectories. 
When it finds such a file, the referenced Go source is built, zipped and deployed to AWS.

## Prerequisites

* Go installed and registered, so you can use the `go build` command
* For authentication against AWS, one of the following must be satisfied:
  * AWS CLI installed and logged in and Region specified through environment variable `AWS_REGION`
  * AWS Credentials and region specified in environment variables
    * `AWS_ACCESS_KEY_ID`
    * `AWS_SECRET_ACCESS_KEY`
    * `AWS_REGION`

## File Structure
```yaml
# Name of the Function used on AWS.
# Must be unique in your region.
name: "hello-world"

# Go file which contains your function code.
# Must be in the same directory
fileName: "hello.go"
```