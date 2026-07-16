# Overview

This repo is a master repo for a an application named BreachLine and all of its supporting infrastructure.

Information about the application can be found in the `README.md` file in the root of the directory.

# Rules

Any tools or applications should be placed inside their own directory inside the `tools` directory in the root of the repo.

Any infrastructure definitions should be placed inside their own directory inside the `infra` directory in the root of the repo.

Any tools created should have words in their directory name separated by - characters. Example: this-is-a-tool

# Tasks

- All currently open tasks are contained in `doc/TODO.md`
- Tasks should be completed from this todo list and checked off as you complete them

# Architecture

## Main application

The main application, BreachLine, is a Wails application, with a Go backend and a frontend written in TypeScript using React as the UI framework.

The application source code is located in the `application` directory in the root of the repo.

## Website

The website source code is located in the `infra/website` directory.

It is a static website hosted on an S3 bucket in front of a CloudFront distribution.

Using this website, users can obtain information about the application, download the application builds and purchase a license for the premium version.

## Payment API (license generator)

The payment API handles the payment flow after payments have been processed by Stripe. It consists of a series of Go lambda functions which handle generating and delivering application license files to customers.

The code for this component can be found in the `infra/payment-api` directory in the root of the repo.

## Sync API

The sync API backs workspace/annotation sync and license-gated auth. Its Go lambda functions live in the `infra/sync-api` directory.

## Deploying infrastructure

All infra (`infra/payment-api`, `infra/sync-api`, `infra/website`) is deployed from the repo-root `Makefile`, driven by a single committed `config.yml` (non-secret) + `secrets.vault.yml` (ansible-vault encrypted). Two environments coexist in the same AWS account, suffixed `-dev`/`-prod` on every resource name + tfstate key; `dev` uses the Stripe sandbox, `prod` the live Stripe API.

- `make init` — bootstrap `config.yml` + `secrets.vault.yml` + `.vault-password-file` (restore the password file from 1Password on a new machine).
- `make edit-secrets` — set `vault.environments.<env>.stripe.{api_key,webhook_secret}`.
- `make deploy ENV=dev` / `make deploy ENV=prod` (or `make deploy-dev` / `deploy-prod`) — render + build lambdas + `terraform apply` each enabled component.
- `make help` lists all targets. Non-secret per-env config is in `config.yml`; secrets in the vault. Rendered `secrets.env` + `infra/<c>/<env>.tfvars` are gitignored.

# Backend

When writing an API, always use Go and the standard http library

# Application

When analysing or making changes to the main application in the `application` directory in the root of the repo. Ensure you first read the readme located at `application/README.md` and the supporting documentation located inside the `application/doc/` directory.

# Frontend

When writing a frontend UI which talks to an API or backend, always use TypeScript using React as the framework.

# Scripts

When writing short scripts that are to be run as a standalone application:

- Always use python
- Always use argparse to accept command line parameters in python applications

# When using python

- Always target python3
- Always create a virtualenv located at `venv` inside the root directory of the specific tool or application
- Always create a requirements.txt file containing every requirement for the tool
- Always separate function definitions by two empty lines
- Always use the python logging package to log debug, error and info information to the console so that the script can be easily debugged

# When creating infrastructure definitions

- Always create a README.md in the same directory as the terraform templates explaining how to use them and what input variables are required
- Always use terraform for managing servers / lambda functions etc
- All AWS resources created by the terraform template should have a tag named project with the value breachline
- Terraform state files should always be stored in the AWS
S3 bucket named `breachline-state-uf308ht4` in the us-east-2 region. The state should always be contained in folder within the bucket named after the infrastructure component, and (for components with dev/prod environments) scoped by environment, such as `payment-api/dev/terraform.tfstate` and `payment-api/prod/terraform.tfstate`
- Always use a lockfile in the same bucket folder as the tfstate file for the state locking
- Always use a Go lambda function for tasks which should run on a schedule or are not interactive. If unsure, ask for clarification before continuing
- Always use ansible for provisioning servers
- If software not installed via a package manager needs to be installed on a server, install them to a directory in the home directory named `software`. On linux it should be placed in `~/software`. This should always be done when installing packages that need to be cloned from git
- If software needs to be symlinked somewhere in the $PATH, symlink it to `~/.usr/local/bin`. Ensure that this is in the $PATH variable.
- Always use ubuntu 25.04 server edition for linux servers, if this isn't available fallback to ubuntu 24.04
- Prefer linux servers over windows
- When chosing a cloud region always choose the region closest to new zealand, such as AWS ap-southeast-2 (sydney) or on digitalocean SYD1 (sydney)
- When choosing a server size, always go with 1 CPU and 2 gigs of ram when possible, or smaller if it would suite the intended workload
- Ensure that all AWS servers allow logins from the following ssh keys:
  - scrappy-ubuntu
  - scrappy-laptop
