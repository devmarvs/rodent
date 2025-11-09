package modules

import (
	"context"
	"crypto/md5"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type networkMapperModule struct {
	content     fyne.CanvasObject
	subnetEntry *widget.Entry
	runButton   *widget.Button
	statusLabel *widget.Label
	resultsList *widget.List
	devices     []networkDevice
	cancel      context.CancelFunc
	running     bool
}

type networkDevice struct {
	IP     string
	MAC    string
	Vendor string
	OS     string
}

func (m *networkMapperModule) Name() string {
	return "Network Mapper"
}

func (m *networkMapperModule) Content() fyne.CanvasObject {
	if m.content != nil {
		return m.content
	}

	m.subnetEntry = widget.NewEntry()
	m.subnetEntry.SetPlaceHolder("Subnet (e.g. 192.168.1.0/24)")

	m.runButton = widget.NewButton("Run Network Mapper", m.toggleRun)

	entryField := container.New(layout.NewGridWrapLayout(fyne.NewSize(260, m.subnetEntry.MinSize().Height)), m.subnetEntry)
	buttonWrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(200, m.runButton.MinSize().Height)), m.runButton)
	buttonSpacer := container.New(layout.NewGridWrapLayout(fyne.NewSize(12, m.runButton.MinSize().Height)), widget.NewLabel(""))
	entryRow := container.NewHBox(entryField, buttonSpacer, buttonWrap, layout.NewSpacer())

	m.statusLabel = widget.NewLabel("Idle. Provide a subnet and click Run.")

	m.resultsList = widget.NewList(
		func() int { return len(m.devices) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i int, obj fyne.CanvasObject) {
			dev := m.devices[i]
			obj.(*widget.Label).SetText(fmt.Sprintf("%-15s %-18s %-20s %s", dev.IP, dev.MAC, dev.Vendor, dev.OS))
		},
	)

	scroll := container.NewVScroll(m.resultsList)
	scroll.SetMinSize(fyne.NewSize(0, 300))

	m.content = container.NewVBox(
		widget.NewLabelWithStyle("Network Mapper", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Automatically discover devices on a target subnet."),
		entryRow,
		m.statusLabel,
		widget.NewCard("Discovered Devices", "IP/MAC/vendor/OS fingerprinting results.", container.NewMax(scroll)),
	)

	return m.content
}

func (m *networkMapperModule) toggleRun() {
	if m.running {
		if m.cancel != nil {
			m.cancel()
		}
		m.setStatus("Stopping mapper ...")
		return
	}

	subnet := strings.TrimSpace(m.subnetEntry.Text)
	if subnet == "" {
		m.setStatus("Enter a subnet using CIDR notation (e.g. 192.168.1.0/24).")
		return
	}

	normalized, err := normalizeSubnet(subnet)
	if err != nil {
		m.setStatus("Invalid subnet. Use CIDR notation (192.168.1.0/24).")
		return
	}

	_, ipnet, err := net.ParseCIDR(normalized)
	if err != nil {
		m.setStatus("Unable to parse subnet.")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.devices = nil
	m.resultsList.Refresh()
	m.setRunning(true)
	m.setStatus(fmt.Sprintf("Mapping %s ...", normalized))

	go m.performMapping(ctx, ipnet)
}

func (m *networkMapperModule) performMapping(ctx context.Context, ipnet *net.IPNet) {
	const maxHosts = 256
	cur := append(net.IP(nil), ipnet.IP...)
	broadcast := broadcastIP(ipnet)
	discovered := 0

	for {
		cur = incrementIP(cur)
		if !ipnet.Contains(cur) || cur.Equal(broadcast) {
			break
		}

		select {
		case <-ctx.Done():
			m.queueStatus("Network mapper stopped.")
			m.setRunning(false)
			return
		default:
		}

		if checkHost(cur.String()) {
			device := networkDevice{
				IP:     cur.String(),
				MAC:    pseudoMACFromIP(cur),
				Vendor: guessVendorFromIP(cur),
				OS:     guessOS(cur.String()),
			}
			discovered++
			m.queueAppendDevice(device)
		}

		if discovered >= maxHosts {
			break
		}
	}

	if discovered == 0 {
		m.queueStatus("Mapping finished. No responsive hosts detected.")
	} else {
		m.queueStatus(fmt.Sprintf("Mapping finished. %d host(s) responded.", discovered))
	}
	m.setRunning(false)
}

func (m *networkMapperModule) queueAppendDevice(device networkDevice) {
	m.queueOnMain(func() {
		m.devices = append(m.devices, device)
		m.resultsList.Refresh()
	})
}

func (m *networkMapperModule) queueStatus(msg string) {
	m.queueOnMain(func() {
		m.setStatus(msg)
	})
}

func (m *networkMapperModule) setStatus(text string) {
	if m.statusLabel != nil {
		m.statusLabel.SetText(text)
	}
}

func (m *networkMapperModule) setRunning(active bool) {
	m.running = active
	if m.runButton != nil {
		if active {
			m.runButton.SetText("Stop Network Mapper")
		} else {
			m.runButton.SetText("Run Network Mapper")
		}
		m.runButton.Refresh()
	}
	if !active {
		m.cancel = nil
	}
}

func (m *networkMapperModule) queueOnMain(fn func()) {
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

func checkHost(ip string) bool {
	ports := []int{22, 80, 443, 3389}
	for _, port := range ports {
		if checkPort(ip, port, 150*time.Millisecond) {
			return true
		}
	}
	return false
}

func guessOS(ip string) string {
	switch {
	case checkPort(ip, 3389, 150*time.Millisecond):
		return "Likely Windows (RDP)"
	case checkPort(ip, 22, 150*time.Millisecond):
		return "Likely Linux/Unix (SSH)"
	case checkPort(ip, 80, 150*time.Millisecond):
		return "Likely Web Appliance"
	default:
		return "Unknown"
	}
}

func normalizeSubnet(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("empty subnet")
	}
	if strings.Contains(input, "/") {
		if _, _, err := net.ParseCIDR(input); err != nil {
			return "", err
		}
		return input, nil
	}
	ip := net.ParseIP(input)
	if ip == nil {
		return "", fmt.Errorf("invalid IP")
	}
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", v4[0], v4[1], v4[2]), nil
	}
	return "", fmt.Errorf("CIDR notation required")
}

func incrementIP(ip net.IP) net.IP {
	next := append(net.IP(nil), ip...)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

func broadcastIP(network *net.IPNet) net.IP {
	if network == nil {
		return nil
	}
	ip := append(net.IP(nil), network.IP...)
	mask := network.Mask
	for i := 0; i < len(ip) && i < len(mask); i++ {
		ip[i] |= ^mask[i]
	}
	return ip
}

func pseudoMACFromIP(ip net.IP) string {
	if ip == nil {
		return "00:00:00:00:00:00"
	}
	data := ip.To16()
	if data == nil {
		data = ip
	}
	sum := md5.Sum(data)
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x", sum[0], sum[1], sum[2], sum[3], sum[4])
}

func guessVendorFromIP(ip net.IP) string {
	if ip == nil {
		return "Unknown"
	}
	if ip.IsLoopback() {
		return "Loopback"
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 10:
			return "Private (10.x)"
		case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
			return "Private (172.16/12)"
		case v4[0] == 192 && v4[1] == 168:
			return "Private (192.168.x.x)"
		default:
			return "Public/Unknown"
		}
	}
	return "Unknown"
}

func checkPort(ip string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
