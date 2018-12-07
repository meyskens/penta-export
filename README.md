Penta Export
============

Penta Export is a project designed to quickly export talk proposals of a specific devroom at FOSDEM into a CSV. Where these can be used in an external review tool.

## Requirements

In order to build this you will need Go 1.11+ installed.

## Configuration

This project uses environment variables to configure it's working. 
```bash
export PENTA_USERNAME="username"
export PENTA_PASSWORD="password"
export PENTA_DEVROOM_ID="693" #Go Devroom ID (look it up in the source of the advanced search page at https://<<PENTAURL>>/search/event)
```

## Usage

Once the configuration is set when running this script it will output CSV content to stdout. It is suggested to catch it in your shell.
```bash
go run main.go | tee devroom
```
