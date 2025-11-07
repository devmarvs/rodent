package modules

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type portStatus struct {
	Port    int
	Service string
	Status  string
}

type scannerModule struct {
	content      fyne.CanvasObject
	targetEntry  *widget.Entry
	scanButton   *widget.Button
	statusLabel  *widget.Label
	detailsLabel *widget.Label
	systemLabel  *widget.Label
	resultsList  *widget.List
	portStatuses []portStatus
	portIndex    map[int]int
	scanCancel   context.CancelFunc
	scanning     bool
}

func (m *scannerModule) Name() string {
	return "Scanner"
}

func (m *scannerModule) Content() fyne.CanvasObject {
	if m.content != nil {
		return m.content
	}

	m.targetEntry = widget.NewEntry()
	m.targetEntry.SetPlaceHolder("Target hostname or IP")

	m.scanButton = widget.NewButton("Scan", m.startScan)
	buttonMin := m.scanButton.MinSize()
	const buttonWidthScale float32 = 1.5
	buttonWidth := fyne.NewSize(buttonMin.Width*buttonWidthScale, buttonMin.Height)

	entryContainer := container.New(layout.NewGridWrapLayout(fyne.NewSize(240, m.targetEntry.MinSize().Height)), m.targetEntry)
	buttonContainer := container.New(layout.NewGridWrapLayout(buttonWidth), m.scanButton)
	entrySpacer := container.New(layout.NewGridWrapLayout(fyne.NewSize(8, m.targetEntry.MinSize().Height)), widget.NewLabel(""))
	formRow := container.NewHBox(entryContainer, entrySpacer, buttonContainer)

	m.detailsLabel = widget.NewLabel("No target selected.")
	m.detailsLabel.Wrapping = fyne.TextWrapWord
	detailsTitle := widget.NewLabelWithStyle("Target Details", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detailsContent := container.NewVBox(detailsTitle, widget.NewSeparator(), m.detailsLabel)
	detailsCard := widget.NewCard("", "", container.NewMax(detailsContent))
	const detailsWidth float32 = 260
	detailsContainer := container.New(layout.NewGridWrapLayout(fyne.NewSize(detailsWidth, detailsCard.MinSize().Height)), detailsCard)

	m.systemLabel = widget.NewLabel("Gathering system info...")
	m.systemLabel.Wrapping = fyne.TextWrapWord
	systemTitle := widget.NewLabelWithStyle("System Fingerprint", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	systemScroll := container.NewVScroll(m.systemLabel)
	const systemHeightScale float32 = 1.0
	systemScroll.SetMinSize(fyne.NewSize(detailsWidth, detailsCard.MinSize().Height*systemHeightScale))
	systemContent := container.NewVBox(systemTitle, widget.NewSeparator(), systemScroll)
	systemCard := widget.NewCard("", "", container.NewMax(systemContent))
	const systemWidth float32 = 260
	systemContainer := container.New(layout.NewGridWrapLayout(fyne.NewSize(systemWidth, systemCard.MinSize().Height)), systemCard)

	const spacerWidth float32 = 8
	boxSpacer := container.New(layout.NewGridWrapLayout(fyne.NewSize(spacerWidth, detailsCard.MinSize().Height)), widget.NewLabel(""))
	boxesRow := container.NewHBox(detailsContainer, boxSpacer, systemContainer)

	overviewLabel := widget.NewLabelWithStyle("Scanner Overview", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	descriptionLabel := widget.NewLabel("Configure and monitor scanning tasks.")

	leftColumn := container.NewVBox(overviewLabel, descriptionLabel, formRow)
	const columnsGap float32 = 16
	columnsSpacer := container.New(layout.NewGridWrapLayout(fyne.NewSize(columnsGap, detailsCard.MinSize().Height)), widget.NewLabel(""))
	headerRow := container.NewHBox(leftColumn, columnsSpacer, boxesRow, layout.NewSpacer())

	m.statusLabel = widget.NewLabel("Enter a hostname or IP address to begin scanning.")

	m.resultsList = widget.NewList(
		func() int { return len(m.portStatuses) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i int, obj fyne.CanvasObject) {
			ps := m.portStatuses[i]
			obj.(*widget.Label).SetText(fmt.Sprintf("%-5d/tcp %-16s %s", ps.Port, ps.Service, ps.Status))
		},
	)

	resultsScroll := container.NewVScroll(m.resultsList)
	resultsScroll.SetMinSize(fyne.NewSize(0, 260))
	resultsCard := widget.NewCard("Scan Results", "Displays status for common TCP ports.", container.NewMax(resultsScroll))

	m.resetDisplayState()

	m.content = container.NewVBox(
		headerRow,
		m.statusLabel,
		resultsCard,
	)

	return m.content
}

func (m *scannerModule) startScan() {
	if m.scanning {
		m.requestStop()
		return
	}

	target := strings.TrimSpace(m.targetEntry.Text)
	if target == "" {
		m.setStatus("Please enter a hostname or IP address.")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.scanCancel = cancel
	m.setScanActive(true)
	m.setStatus(fmt.Sprintf("Scanning %s...", target))
	m.updateTargetDetails(target)
	m.populateSystemDetails()
	m.initPortStatuses("pending")

	go m.performScan(ctx, target)
}

func (m *scannerModule) requestStop() {
	if !m.scanning {
		return
	}
	if m.scanCancel != nil {
		m.scanCancel()
	}
	m.setStatus("Stopping current scan...")
}

func (m *scannerModule) performScan(ctx context.Context, target string) {
	ports := portCatalog()
	canceled := false

scanLoop:
	for _, def := range ports {
		select {
		case <-ctx.Done():
			canceled = true
			break scanLoop
		default:
		}

		port := def.Port
		address := net.JoinHostPort(target, strconv.Itoa(port))

		m.queueOnMain(func() {
			m.setPortStatus(port, "scanning...")
		})

		state := m.scanPort(address)

		m.queueOnMain(func() {
			m.setPortStatus(port, state)
		})
	}

	m.queueOnMain(func() {
		if canceled {
			m.setStatus("Scan stopped.")
		} else {
			m.setStatus(fmt.Sprintf("Scan complete for %s (%d ports).", target, len(ports)))
		}
		m.setScanActive(false)
		m.scanCancel = nil
	})
}

func (m *scannerModule) scanPort(address string) string {
	conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "filtered (timeout)"
		}

		return "closed"
	}

	conn.Close()
	return "open"
}

func (m *scannerModule) setStatus(text string) {
	if m.statusLabel != nil {
		m.statusLabel.SetText(text)
	}
}

func (m *scannerModule) updateTargetDetails(target string) {
	if m.detailsLabel == nil {
		return
	}

	if target == "" {
		m.detailsLabel.SetText("No target selected.")
		return
	}

	lines := []string{
		fmt.Sprintf("Target: %s", target),
	}

	if ips, err := net.LookupIP(target); err == nil && len(ips) > 0 {
		var ipStrings []string
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				ipStrings = append(ipStrings, ipv4.String())
			} else {
				ipStrings = append(ipStrings, ip.String())
			}
			if len(ipStrings) == 3 {
				break
			}
		}
		lines = append(lines, fmt.Sprintf("IP(s): %s", strings.Join(ipStrings, ", ")))
	} else {
		lines = append(lines, "IP(s): unavailable")
	}

	lines = append(lines, fmt.Sprintf("Last scan: %s", time.Now().Format(time.RFC1123)))

	m.detailsLabel.SetText(strings.Join(lines, "\n"))
}

func (m *scannerModule) populateSystemDetails() {
	if m.systemLabel == nil {
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	lines := []string{
		fmt.Sprintf("Hostname: %s", hostname),
		fmt.Sprintf("OS / Arch: %s / %s", runtime.GOOS, runtime.GOARCH),
		fmt.Sprintf("Go Version: %s", runtime.Version()),
		fmt.Sprintf("CPU Cores: %d", runtime.NumCPU()),
		fmt.Sprintf("GOMAXPROCS: %d", runtime.GOMAXPROCS(0)),
		fmt.Sprintf("Time: %s", time.Now().Format(time.RFC1123)),
	}

	m.systemLabel.SetText(strings.Join(lines, "\n"))
}

func (m *scannerModule) setPortStatus(port int, status string) {
	if idx, ok := m.portIndex[port]; ok {
		m.portStatuses[idx].Status = status
		m.refreshResults()
	}
}

func (m *scannerModule) refreshResults() {
	if m.resultsList != nil {
		m.resultsList.Refresh()
	}
}

func (m *scannerModule) queueOnMain(fn func()) {
	if app := fyne.CurrentApp(); app != nil {
		if drv := app.Driver(); drv != nil {
			if runner, ok := drv.(interface{ RunOnMain(func()) }); ok {
				runner.RunOnMain(fn)
				return
			}
		}
	}

	fn()
}

func (m *scannerModule) resetDisplayState() {
	m.scanCancel = nil
	m.setScanActive(false)
	m.clearPortStatuses()
	m.setStatus("Enter a hostname or IP address to begin scanning.")
	m.updateTargetDetails("")
	if m.systemLabel != nil {
		m.systemLabel.SetText("System info not yet captured.")
	}
}

func (m *scannerModule) setScanActive(active bool) {
	m.scanning = active
	if m.scanButton != nil {
		if active {
			m.scanButton.SetText("Stop")
		} else {
			m.scanButton.SetText("Scan")
		}
		m.scanButton.Refresh()
	}
}

func (m *scannerModule) initPortStatuses(defaultStatus string) {
	defs := portCatalog()
	m.portStatuses = make([]portStatus, len(defs))
	m.portIndex = make(map[int]int, len(defs))
	for i, def := range defs {
		m.portStatuses[i] = portStatus{
			Port:    def.Port,
			Service: def.Service,
			Status:  defaultStatus,
		}
		m.portIndex[def.Port] = i
	}
	m.refreshResults()
}

func (m *scannerModule) clearPortStatuses() {
	m.portStatuses = nil
	m.portIndex = nil
	m.refreshResults()
}

type portDefinition struct {
	Port    int
	Service string
}

func portCatalog() []portDefinition {
	return []portDefinition{
		{20, "FTP Data"},
		{21, "FTP Control"},
		{22, "SSH"},
		{23, "Telnet"},
		{25, "SMTP"},
		{53, "DNS"},
		{80, "HTTP"},
		{110, "POP3"},
		{143, "IMAP"},
		{194, "IRC"},
		{443, "HTTPS"},
		{465, "SMTPS"},
		{587, "Mail Submission"},
		{993, "IMAPS"},
		{995, "POP3S"},
		{1433, "MS SQL"},
		{1521, "Oracle DB"},
		{2049, "NFS"},
		{2375, "Docker API"},
		{27017, "MongoDB"},
		{3306, "MySQL"},
		{3389, "RDP"},
		{5432, "PostgreSQL"},
		{5900, "VNC"},
		{6379, "Redis"},
		{8080, "HTTP Alt"},
		{8443, "HTTPS Alt"},
		{9000, "Custom"},
	}
}
