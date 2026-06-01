package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed goindex.js
var embeddedJS string

//go:embed goconfig.yaml
var embeddedConfig []byte

//go:embed goindex.html
var embeddedHTML string

type ConfigSettings struct {
	PageTitle       string
	PageDescription string
	Favicon         string
	Styles          []string
	Scripts         []string
}

type FileNode struct {
	Name     string      `json:"name"`
	IsDir    bool        `json:"is_dir"`
	Size     int64       `json:"size"`
	ModTime  time.Time   `json:"mod_time"`
	Children []*FileNode `json:"children,omitempty"`
}

// --- Helper Functions ---

func buildTree(path string) (*FileNode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	node := &FileNode{
		Name:    info.Name(),
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return node, nil
		}
		for _, entry := range entries {
			if entry.Name() == "goserver.yaml" {
				continue
			}
			childNode, err := buildTree(filepath.Join(path, entry.Name()))
			if err == nil {
				node.Children = append(node.Children, childNode)
			}
		}
	}
	return node, nil
}

func getDirStructure(path string) []*FileNode {
	var nodes []*FileNode
	entries, _ := os.ReadDir(path)
	for _, e := range entries {
		if e.Name() == "goserver.yaml" {
			continue
		}
		info, _ := e.Info()
		nodes = append(nodes, &FileNode{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	return nodes
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "localhost"
}

func isPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

func generateConfigFile(targetDir string) {
	configPath := filepath.Join(targetDir, "goserver.yaml")
	if _, err := os.Stat(configPath); err == nil {
		log.Fatalf("Error: 'goserver.yaml' already exists. Aborting.\n")
	}
	os.WriteFile(configPath, embeddedConfig, 0644)
	fmt.Printf("Successfully generated template configuration at: %s\n", configPath)
}

func cleanValue(val string) string {
	val = strings.TrimSpace(val)
	if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
		(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
		if len(val) >= 2 {
			val = val[1 : len(val)-1]
		}
	}
	return val
}

func parseInlineArray(val string) []string {
	var items []string
	val = strings.TrimSpace(val)
	if !strings.HasPrefix(val, "[") || !strings.HasSuffix(val, "]") {
		return items
	}
	content := val[1 : len(val)-1]
	parts := strings.Split(content, ",")
	for _, p := range parts {
		cleaned := cleanValue(p)
		if cleaned != "" {
			items = append(items, cleaned)
		}
	}
	return items
}

func parseYAMLFile(filePath string) ConfigSettings {
	var settings ConfigSettings
	file, err := os.Open(filePath)
	if err != nil {
		return settings
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	currentBlock := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-") {
			trimmedLine := strings.TrimSpace(line[1:])
			if currentBlock == "page_info" {
				parts := strings.SplitN(trimmedLine, ":", 2)
				if len(parts) == 2 {
					subKey := strings.ToLower(strings.TrimSpace(parts[0]))
					subVal := cleanValue(parts[1])
					switch subKey {
					case "page_title":
						settings.PageTitle = subVal
					case "page_description":
						settings.PageDescription = subVal
					case "favicon":
						settings.Favicon = subVal
					}
				}
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "page_info":
			currentBlock = "page_info"
		case "styles":
			currentBlock = "styles"
			settings.Styles = append(settings.Styles, parseInlineArray(val)...)
		case "scripts":
			currentBlock = "scripts"
			settings.Scripts = append(settings.Scripts, parseInlineArray(val)...)
		}
	}
	return settings
}

func buildHTMLHeadTag(cfg ConfigSettings) string {
	var sb strings.Builder
	title := "GoServer Live Workspace"
	if cfg.PageTitle != "" {
		title = cfg.PageTitle
	}
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", title))
	if cfg.PageDescription != "" {
		sb.WriteString(fmt.Sprintf("<meta name=\"description\" content=\"%s\">\n", cfg.PageDescription))
	}
	if cfg.Favicon != "" {
		sb.WriteString(fmt.Sprintf("<link rel=\"icon\" href=\"%s\">\n", cfg.Favicon))
	}
	for _, sheet := range cfg.Styles {
		sb.WriteString(fmt.Sprintf("<link rel=\"stylesheet\" href=\"%s\">\n", sheet))
	}
	sb.WriteString("<script>" + embeddedJS + "</script>\n")
	for _, sc := range cfg.Scripts {
		sb.WriteString(fmt.Sprintf("<script src=\"%s\"></script>\n", sc))
	}
	return sb.String()
}

func printHelp() {
	helpText := `
GoServer - A portable web server and file browser utility built in Go.

Usage:
  goserver [OPTIONS]

Options:
  -dir <path>        Target folder directory to serve files from.
                     Defaults to current directory.
  -port <port> 	     The explicit port to run the local server on.
                     Defaults to the first available port starting from 8080.
  --writable         Enables file uploads to the server.

Flags:
  --config           Generates a blank template 'goserver.yaml' file in the target directory.
  --help             Displays this CLI usage instructions manual.
`
	fmt.Print(helpText)
}

// --- Main Function ---

func main() {
	dirFlag := flag.String("dir", ".", "The directory path to serve files from")
	portFlag := flag.Int("port", 0, "The explicit port to run the local server on")
	configFlag := flag.Bool("config", false, "Generate a blank template goserver.yaml file")
	writableFlag := flag.Bool("writable", false, "Enable file uploads")
	helpFlag := flag.Bool("help", false, "Display help information")
	flag.Parse()

	if *helpFlag {
		printHelp()
		return
	}

	absDir, err := filepath.Abs(*dirFlag)
	if err != nil {
		log.Fatalf("Error resolving path: %v\n", err)
	}

	if *configFlag {
		generateConfigFile(absDir)
		return
	}

	tmpl, err := template.New("index").Parse(embeddedHTML)
	if err != nil {
		log.Fatalf("Error parsing template: %v\n", err)
	}

	config := ConfigSettings{}
	yamlPath := filepath.Join(absDir, "goserver.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		config = parseYAMLFile(yamlPath)
	}

	finalPort := *portFlag
	if finalPort == 0 {
		startingPort := 8080
		for {
			if isPortAvailable(startingPort) {
				finalPort = startingPort
				break
			}
			startingPort++
		}
	} else if !isPortAvailable(finalPort) {
		log.Fatalf("Error: Requested port %d is already in use.\n", finalPort)
	}

	mux := http.NewServeMux()
	mux.Handle("/_serve/", http.StripPrefix("/_serve/", http.FileServer(http.Dir(absDir))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !*writableFlag {
				http.Error(w, "Uploads are disabled. Start server with --writable to enable.", http.StatusForbidden)
				return
			}
			r.ParseMultipartForm(50 << 20)
			targetFolder := r.FormValue("target_folder")
			safeFolder := filepath.Clean(targetFolder)
			if strings.Contains(safeFolder, "..") {
				safeFolder = "."
			}
			uploadPath := filepath.Join(absDir, safeFolder)
			os.MkdirAll(uploadPath, 0755)
			files := r.MultipartForm.File["upload_assets"]
			for _, fh := range files {
				src, _ := fh.Open()
				dst, _ := os.Create(filepath.Join(uploadPath, filepath.Base(fh.Filename)))
				io.Copy(dst, src)
				src.Close()
				dst.Close()
			}
			return
		}

		fullTree, _ := buildTree(absDir)
		fullTreeJSON, _ := json.Marshal(fullTree)

		requestedPath := strings.TrimPrefix(r.URL.Path, "/")
		if requestedPath == "" {
			requestedPath = "."
		}
		currentPath := filepath.Join(absDir, requestedPath)
		if _, err := os.Stat(currentPath); err != nil {
			currentPath = absDir
		}

		dirData := getDirStructure(currentPath)
		currentDirJSON, _ := json.Marshal(dirData)

		// Added IsWritable field here
		data := struct {
			HeadElements template.HTML
			GoStructure  template.JS
			GoDir        template.JS
			IsWritable   bool
		}{
			HeadElements: template.HTML(buildHTMLHeadTag(config)),
			GoStructure:  template.JS(fullTreeJSON),
			GoDir:        template.JS(currentDirJSON),
			IsWritable:   *writableFlag,
		}
		tmpl.Execute(w, data)
	})

	fmt.Println("---------------------------------------------------------")
	fmt.Printf(" Serving Directory : %s\n", absDir)
	fmt.Printf(" Writable Mode     : %v\n", *writableFlag)
	fmt.Printf(" Local Machine URL : http://localhost:%d\n", finalPort)
	fmt.Printf(" Network LAN URL   : http://%s:%d\n", getLocalIP(), finalPort)
	fmt.Println("---------------------------------------------------------")
	fmt.Println(" Press Ctrl+C to terminate the goserver instance...")

	http.ListenAndServe(fmt.Sprintf(":%d", finalPort), mux)
}
