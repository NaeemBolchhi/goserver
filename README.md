# GoServer

A lightweight, portable web server and file browser utility built in Go. This tool allows you to instantly serve a directory over HTTP, providing a clean interface to browse files and, optionally, upload content.

This tool was made with the assistance of **Gemini**.

---

## Features

* **Embedded Assets:** CSS, JS, and HTML templates are embedded directly into the binary.
* **Customizable:** Support for `goserver.yaml` to inject custom metadata, styles, and scripts.
* **Flexible Serving:** Auto-detects available ports; also allows explicit port binding.
* **Optional Write Access:** Enable file uploads via the `--writable` flag.
* **Zero Dependencies:** Designed as a single-binary utility for portability.

---

## Usage

### Basic Commands

* **Default:** `goserver.exe`
* **Set Directory:** `goserver.exe -dir "C:\Something"`
* **Set Port:** `goserver.exe -port 5000`

---

### Options

| Flag | Description |
| --- | --- |
| `-dir <path>` | Target directory to serve (defaults to `.`). |
| `-port <port>` | Specify the port (defaults to first available starting at 8080). |
| `--writable` | Enables file uploads to the server. |
| `--config` | Generates a `goserver.yaml` file in the target directory. |
| `--help` | Displays the full help manual. |

---

## License

This project is licensed under the [**MIT License**](./LICENSE).