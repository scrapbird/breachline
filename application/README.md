# README

This is the main BreachLine application. Specific implementation details can be found in the `doc` directory.

## Live Development

To run in live development mode, run `wails dev -tags webkit2_42` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Dependencies

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
sudo apt-get update
sudo apt-get install gcc libgtk-3-dev libwebkit2gtk-4.1-dev
```

## Building

To build a redistributable, production mode package, use `wails build -webview2 browser -tags webkit2_42`.

# Documentation

Documentation can be found in the [doc](./doc) directory.
