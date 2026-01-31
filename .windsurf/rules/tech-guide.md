---
trigger: always_on
---

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
S3 bucket named `scrappy-tfstate` in the ap-southeast-2 region. The state should always be contained in folder within the bucket named after the infrastructure component such as `order_processor/terraform.tfstate`
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
