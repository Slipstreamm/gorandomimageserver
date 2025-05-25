package main

import (
	"fmt"
	"html/template" // Added for generating HTML
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImageSource represents a collection of images with its configuration
type ImageSource struct {
	Name         string // Display name (e.g., "Kasane Teto", "Needy Streamer Overload")
	ID           string // URL identifier (e.g., "teto", "nso")
	Directory    string // File system path to images
	StaticPrefix string // URL prefix for serving static images
	RandomPath   string // URL path for random image redirect
	WebPageName  string // Name of the HTML file for the viewer
	WebPath      string // URL path for the viewer webpage
}

// imageSources defines all available image collections
var imageSources = map[string]ImageSource{
	"teto": {
		Name:         "Kasane Teto",
		ID:           "teto",
		Directory:    "/srv/http/downloaded_booru_images_api/kasane_teto",
		StaticPrefix: "/teto/static/",
		RandomPath:   "/teto",
		WebPageName:  "teto-web.html",
		WebPath:      "/teto-web",
	},
	"nso": {
		Name:         "Needy Streamer Overload",
		ID:           "nso",
		Directory:    "/srv/http/downloaded_booru_images_api/needy_girl_overdose",
		StaticPrefix: "/nso/static/",
		RandomPath:   "/nso",
		WebPageName:  "nso-web.html",
		WebPath:      "/nso-web",
	},
	"public_http": {
		Name:         "Public HTTP Root",
		ID:           "public_http",
		Directory:    "/srv/http/",
		StaticPrefix: "/public/",
	},
}

// imageExtensions maps file extensions to their MIME types.
var imageExtensions = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
}

// Template for the static index page (image list)
var indexTemplate = template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>Image Index</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f4f4f4; color: #333; }
        h1 { color: #5a5a5a; }
        ul { list-style-type: none; padding: 0; }
        li { margin: 5px 0; }
        a { text-decoration: none; color: #007bff; }
        a:hover { text-decoration: underline; }
        .container { max-width: 800px; margin: auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
    </style>
</head>
<body>
    <div class="container">
        <h1>Available Images</h1>
        <p>Click on an image name to view it. Total images: {{len .}}</p>
        <ul>
            {{range .}}
            <li><a href="{{.StaticPath}}{{.FileName}}">{{.FileName}}</a></li>
            {{end}}
        </ul>
    </div>
</body>
</html>
`))

// Struct to pass data to the index template
type IndexPageData struct {
	FileName   string
	StaticPath string
}

// init is called before main to seed the random number generator.
func init() {
	rand.Seed(time.Now().UnixNano())
}

// listAvailableImages scans the specified directory and returns a slice of valid image filenames.
func listAvailableImages(imageDir string) ([]string, error) {
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", imageDir, err)
	}

	var images []string
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if _, ok := imageExtensions[ext]; ok {
				images = append(images, entry.Name())
			}
		}
	}
	return images, nil
}

// createRandomImageRedirectHandler creates a handler for random image redirects for a specific image source.
func createRandomImageRedirectHandler(source ImageSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		images, err := listAvailableImages(source.Directory)
		if err != nil {
			log.Printf("Error listing available images for %s: %v", source.Name, err)
			http.Error(w, "Internal Server Error: Could not list images.", http.StatusInternalServerError)
			return
		}

		if len(images) == 0 {
			log.Printf("No images found in %s", source.Directory)
			http.Error(w, "No images found.", http.StatusNotFound)
			return
		}

		randomIndex := rand.Intn(len(images))
		randomImageName := images[randomIndex]

		// Construct the redirect URL
		redirectURL := source.StaticPrefix + randomImageName

		// Perform a temporary redirect (302)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		log.Printf("Redirecting to: %s for a random %s image", redirectURL, source.Name)
	}
}

// createStaticIndexHandler creates a handler for serving an HTML page listing all available images for a specific image source.
func createStaticIndexHandler(source ImageSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		images, err := listAvailableImages(source.Directory)
		if err != nil {
			log.Printf("Error listing available images for %s index: %v", source.Name, err)
			http.Error(w, "Internal Server Error: Could not list images.", http.StatusInternalServerError)
			return
		}

		if len(images) == 0 {
			log.Printf("No images found in %s to display in index.", source.Directory)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintln(w, "<h1>No images available</h1>")
			return
		}

		var pageData []IndexPageData
		for _, imgName := range images {
			pageData = append(pageData, IndexPageData{FileName: imgName, StaticPath: source.StaticPrefix})
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = indexTemplate.Execute(w, pageData)
		if err != nil {
			log.Printf("Error executing HTML template: %v", err)
			http.Error(w, "Internal Server Error: Could not render index page.", http.StatusInternalServerError)
		}
		log.Printf("Served image index page for %s", source.StaticPrefix)
	}
}

// createStaticFileHandler creates a handler for serving static files for a specific image source.
func createStaticFileHandler(source ImageSource) http.HandlerFunc {
	indexHandler := createStaticIndexHandler(source)

	return func(w http.ResponseWriter, r *http.Request) {
		// If the request path is exactly the staticPathPrefix, serve the index.
		if r.URL.Path == source.StaticPrefix {
			indexHandler(w, r)
			return
		}

		// Otherwise, try to serve a static file.
		imageName := strings.TrimPrefix(r.URL.Path, source.StaticPrefix)

		if imageName == "" {
			http.NotFound(w, r)
			return
		}

		// Basic path traversal prevention
		if filepath.Base(imageName) != imageName {
			log.Printf("Attempted path traversal: %s", imageName)
			http.Error(w, "Bad Request: Invalid image name.", http.StatusBadRequest)
			return
		}

		imagePath := filepath.Join(source.Directory, imageName)

		ext := strings.ToLower(filepath.Ext(imageName))
		contentType, ok := imageExtensions[ext]
		if !ok {
			log.Printf("Unsupported image type requested: %s (path: %s)", ext, imagePath)
			http.NotFound(w, r)
			return
		}

		img, err := os.Open(imagePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("Image file not found: %s", imagePath)
				http.NotFound(w, r)
			} else {
				log.Printf("Error opening image file %s: %v", imagePath, err)
				http.Error(w, "Internal Server Error: Could not open image file.", http.StatusInternalServerError)
			}
			return
		}
		defer img.Close()

		fileInfo, err := img.Stat()
		if err != nil {
			log.Printf("Error getting file info for %s: %v", imagePath, err)
			http.Error(w, "Internal Server Error: Could not get image file info.", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, imageName, fileInfo.ModTime(), img)
		log.Printf("Served static %s image: %s", source.Name, imageName)
	}
}

// createWebHandler creates a handler for serving the web page for a specific image source.
func createWebHandler(source ImageSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Serve the HTML file.
		// http.ServeFile will set the Content-Type based on the file extension.
		// It assumes the HTML file is in the current working directory of the server.
		_, err := os.Stat(source.WebPageName)
		if os.IsNotExist(err) {
			log.Printf("HTML file not found: %s", source.WebPageName)
			http.NotFound(w, r)
			return
		} else if err != nil {
			log.Printf("Error checking HTML file %s: %v", source.WebPageName, err)
			http.Error(w, "Internal Server Error: Could not check HTML file.", http.StatusInternalServerError)
			return
		}

		http.ServeFile(w, r, source.WebPageName)
		log.Printf("Served HTML page: %s at %s", source.WebPageName, source.WebPath)
	}
}

// corsMiddleware adds CORS headers to the response.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		// Allow all origins, for more restrictive settings, replace "*" with your specific domain
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// Allowed methods
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		// Allowed headers
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests (OPTIONS method)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK) // Or http.StatusNoContent
			return
		}

		// Call the next handler in the chain
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Check if all image directories exist
	for id, source := range imageSources {
		if _, err := os.Stat(source.Directory); os.IsNotExist(err) {
			log.Printf("Warning: Image directory '%s' for %s does not exist. The %s endpoints might not work.", source.Directory, source.Name, id)
		} else if err != nil {
			log.Printf("Warning: Error checking image directory '%s' for %s: %v", source.Directory, source.Name, err)
		}

		// Check if the HTML file exists (optional, but good for startup feedback)
		// Only check if WebPageName is not empty
		if source.WebPageName != "" {
			if _, err := os.Stat(source.WebPageName); os.IsNotExist(err) {
				log.Printf("Warning: HTML file '%s' not found in the current directory. The %s endpoint might not work.", source.WebPageName, source.WebPath)
			} else if err != nil {
				log.Printf("Warning: Error checking for HTML file '%s': %v", source.WebPageName, err)
			}
		}
	}

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Register handlers for each image source
	for _, source := range imageSources {
		if source.RandomPath != "" {
			mux.HandleFunc(source.RandomPath, createRandomImageRedirectHandler(source))
		}
		if source.StaticPrefix != "" {
			mux.HandleFunc(source.StaticPrefix, createStaticFileHandler(source))
		}
		if source.WebPath != "" {
			mux.HandleFunc(source.WebPath, createWebHandler(source))
		}
	}

	// Create a main index page that lists all available collections
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, "<html><head><title>Image Collections</title></head><body>")
		fmt.Fprintln(w, "<h1>Available Image Collections</h1>")
		fmt.Fprintln(w, "<ul>")

		for _, source := range imageSources {
			fmt.Fprintf(w, "<li><strong>%s</strong><ul>", source.Name)
			if source.RandomPath != "" {
				fmt.Fprintf(w, "<li><a href=\"%s\">Random Image</a></li>", source.RandomPath)
			}
			if source.StaticPrefix != "" {
				fmt.Fprintf(w, "<li><a href=\"%s\">Image Index</a></li>", source.StaticPrefix)
			}
			if source.WebPath != "" {
				fmt.Fprintf(w, "<li><a href=\"%s\">Web Viewer</a></li>", source.WebPath)
			}
			fmt.Fprintln(w, "</ul></li>")
		}

		fmt.Fprintln(w, "</ul></body></html>")
	})

	// Wrap the mux with the CORS middleware
	handlerWithCORS := corsMiddleware(mux)

	port := "5003"
	log.Printf("Starting server on port %s", port)
	log.Printf("Visit http://localhost:%s/ to see all available image collections", port)

	for _, source := range imageSources {
		log.Printf("  %s:", source.Name)
		if source.RandomPath != "" {
			log.Printf("    Random: http://localhost:%s%s", port, source.RandomPath)
		}
		if source.StaticPrefix != "" {
			log.Printf("    Index:  http://localhost:%s%s", port, source.StaticPrefix)
		}
		if source.WebPath != "" {
			log.Printf("    Web:    http://localhost:%s%s", port, source.WebPath)
		}
	}

	// Start the server with the CORS-enabled handler
	if err := http.ListenAndServe(":"+port, handlerWithCORS); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
