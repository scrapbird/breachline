# Overview

This repo is a master repo for a an application named BreachLine and all of its supporting infrastructure.

Information about the application can be found in the `README.md` file in the root of the directory.

# Rules

Any tools or applications should be placed inside their own directory inside the `tools` directory in the root of the repo.

Any infrastructure definitions should be placed inside their own directory inside the `infra` directory in the root of the repo.

Any tools created should have words in their directory name separated by - characters. Example: this-is-a-tool

# Architecture

## Main application

The main application, BreachLine, is a Wails application, with a Go backend and a frontend written in TypeScript using React as the UI framework.

The application source code is located in the `application` directory in the root of the repo.

## Website

The website source code is located in the `infra/website` directory.

It is a static website hosted on an S3 bucket in front of a CloudFront distribution.

Using this website, users can obtain information about the application, download the application builds and purchase a license for the premium version.

## License generator

The license generator handles the payment flow after payments have been processed by Stripe. It consists of a series of Go lambda functions which handle generating and delivering application license files to customers.

the code for this component can be found in the `infra/order_processor` directory in the root of the repo.

