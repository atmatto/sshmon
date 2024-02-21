package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

//go:embed templates/*
var templatesFS embed.FS
var templates *template.Template = template.Must(template.ParseFS(templatesFS, "templates/*"))

//go:embed static/*
var staticFS embed.FS

// data for rendering the pages
type data struct {
	Page string // Current page name

	Time string // Current time
	Ip template.HTML // All IP addresses
	Devices template.HTML // All network devices
	Active string
	Enabled string
	Log string
	Config string
	Fingerprints template.HTML // Fingerprints of all public keys
	Users template.HTML// Active ssh connections
	Port string // ssh port
}

func cmd(command ...string) string {
	c := exec.Command(command[0], command[1:]...)
	out, err := c.Output()
	if err != nil {
		_, ok := err.(*exec.ExitError)
		if !ok {
			log.Println("Command error:", err)
			return string(out)
		}
	}
	return strings.TrimSpace(string(out))
}

func getData() (d data) {
	d.Time = cmd("date", "+%Y-%m-%d %T")
	d.Ip = template.HTML(cmd("bash", "-c", "nmcli -f=IP4.ADDRESS,GENERAL.DEVICE device show | cut -d ' ' -f 2- | sed 's/^ *//' | sed -z -r 's/\\n(\\n|$)/)\\n/g;s/\\n([^\\n]*)\\)/ (\\1)\\n/g' | sed 's/^ .*//;/^$/d' | sed -r 's/^(.*)$/<li>\\0<\\/li>/'"))
	d.Devices = template.HTML(cmd("bash", "-c", "nmcli device status | sed -r 's/  +/<\\/td><td>/g;s/^/<tr><td>/' | sed -r 's/<td>$/<\\/tr>/;1s/td/th/g;s/--/\\&mdash;/g'"))
	d.Active = cmd("systemctl", "is-active", "ssh")
	d.Enabled = cmd("systemctl", "is-enabled", "ssh")
	socketActivated := cmd("bash", "-c", "systemctl show --property=TriggeredBy ssh | sed 's/.*=//'")
	if len(socketActivated) != 0 {
		d.Enabled = "socket-activated"
	}
	d.Log = cmd("journalctl", "-ussh.service", "-b-0", "--no-hostname")
	d.Config = cmd("cat", "/etc/ssh/sshd_config")
	d.Fingerprints = template.HTML(strings.ReplaceAll(cmd("bash", "-c", "ls /etc/ssh/*pub | xargs -I{} ssh-keygen -lv -f {} | sed -r 's/^([[:digit:]])/\\n\\1/'"), "\n\n", "</pre><pre>"))
	d.Users = template.HTML(cmd("bash", "-c", "who | grep -v '(:0)' | sed -r 's/  +/<\\/td><td>/g;s/^/<tr><td>/;s/$/<\\/td><\\/tr>/'"))
	// d.Port = cmd("bash", "-c", "(grep '^[^#]*Port ' /etc/ssh/sshd_config || echo 22) | sed -r 's/Port *//'")
	d.Port = cmd("bash", "-c", "(grep ListenStream /lib/systemd/system/ssh.socket || grep '^[^#]*Port ' /etc/ssh/sshd_config || echo 22) | sed -r 's/^[^[:digit:]]*//'")
	return
}

func handler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		http.Redirect(w, r, "/status.html", http.StatusFound)
		return
	}
	if templates.Lookup(path) == nil {
		http.NotFound(w, r)
		return
	}
	d := getData()
	d.Page = strings.TrimSuffix(path, ".html")
	d.Page = strings.ToTitle(string(d.Page[0])) + d.Page[1:]
	err := templates.ExecuteTemplate(w, path, d)
	if err != nil {
		log.Println("Template error:", err)
	}
}

func main() {
	port := "8012"
	
	http.HandleFunc("/", handler)
	http.Handle("/static/", http.FileServer(http.FS(staticFS)))

	ip := cmd("bash", "-c", "nmcli -f=IP4.ADDRESS,GENERAL.DEVICE device show | cut -d ' ' -f 2- | sed 's/^ *//' | sed -z -r 's/\\n(\\n|$)/)\\n/g;s/\\n([^\\n]*)\\)/ (\\1)\\n/g' | sed 's/^ .*//;/^$/d'")

	log.Println("Starting the server:\n" + ip)
	log.Println("Port number: " + port)

	log.Fatal(http.ListenAndServe(":" + port, nil))
}
