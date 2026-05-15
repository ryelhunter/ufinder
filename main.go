// ufinder.go
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner" // NECESSÁRIO: go get github.com/briandowns/spinner
	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
)

// CONFIGURAÇÃO
const MaxConcurrentTools = 1
const resumeStateFileName = ".ufinder-resume.json"

var defaultToolsOrder = []string{
	"waymore",
	"waybackurls",
	"gau",
	"gau_subs",
	"xurlfind3r",
	"urlscan",
	"urlfinder",
	"ducker",
}

type resumeState struct {
	ListFile     string   `json:"list_file"`
	OutputFolder string   `json:"output_folder,omitempty"`
	Completed    []string `json:"completed"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

// --- HELPERS VISUAIS ---

// Ícones e cores modernas
var (
	iconCheck  = color.New(color.FgGreen, color.Bold).Sprint("✔")
	iconFire   = color.New(color.FgHiYellow).Sprint("⚡")
	iconBox    = color.New(color.FgCyan).Sprint("📦")
	iconSearch = color.New(color.FgHiBlue).Sprint("🔎")

	colorTool = color.New(color.FgHiWhite, color.Bold).SprintFunc()
	colorTime = color.New(color.FgHiBlack).SprintFunc() // Cinza escuro para o tempo
	colorNew  = color.New(color.FgHiGreen, color.Bold).SprintFunc()
	colorZero = color.New(color.FgHiBlack).SprintFunc() // Discreto se for zero
)

func printBanner() {
	// Limpa a tela antes de começar (opcional, remove se não gostar)
	fmt.Print("\033[H\033[2J")

	myFigure := figure.NewFigure("UFINDER", "slant", true)
	color.Cyan(myFigure.String())
	fmt.Println(color.New(color.FgHiBlack).Sprint("\n   by Gilson Oliveira"))
	fmt.Println("")
}

func printHeader(domain, folder string, currentTarget int, totalTargets int) {
	targetLine := domain
	if totalTargets > 0 {
		targetLine = fmt.Sprintf("[%d/%d] %s", currentTarget, totalTargets, domain)
	}

	fmt.Printf("   %s Target: %s\n", iconFire, color.HiWhiteString(targetLine))
	fmt.Printf("   %s Output: %s\n", iconBox, color.HiWhiteString(folder))
	fmt.Println(strings.Repeat(color.HiBlackString("─"), 60))
	fmt.Println("")
}

func printTargetSeparator() {
	fmt.Println("")
	fmt.Println(color.HiBlackString(strings.Repeat("=", 60)))
	fmt.Println("")
}

func printSectionBox(title string) {
	fmt.Println(color.HiCyanString("┌──────────────────────────────────────────────┐"))
	fmt.Printf("│  %s%s│\n", color.HiWhiteString(title), strings.Repeat(" ", 42-len(title)))
	fmt.Println(color.HiCyanString("└──────────────────────────────────────────────┘"))
}

func printElapsedBox(title string, elapsed time.Duration) {
	fmt.Println(color.HiBlackString("┌──────────────────────────────────────────────┐"))
	fmt.Printf("│  %s%s│\n", color.HiWhiteString(title), strings.Repeat(" ", 42-len(title)))
	fmt.Println(color.HiBlackString("├──────────────────────────────────────────────┤"))
	fmt.Printf("│  Elapsed Time       : %-22s │\n", elapsed.Round(time.Second))
	fmt.Println(color.HiBlackString("└──────────────────────────────────────────────┘"))
	fmt.Println("")
}

// --- HELPERS LÓGICOS ---

func fileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func shellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func isJSURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return strings.HasSuffix(strings.ToLower(parsed.Path), ".js")
}

func normalizeJSURL(rawURL string) (string, bool) {
	if !isJSURL(rawURL) {
		return "", false
	}

	return normalizeUniqueURL(rawURL)
}

func normalizeUniqueURL(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", false
	}

	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if port != "" && port != "80" && port != "443" {
		host = host + ":" + port
	}

	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}

	return host + path, true
}

func normalizeHost(rawValue string) (string, bool) {
	rawValue = strings.TrimSpace(rawValue)
	if rawValue == "" {
		return "", false
	}

	if !strings.Contains(rawValue, "://") {
		rawValue = "https://" + rawValue
	}

	parsed, err := url.Parse(rawValue)
	if err != nil || parsed.Host == "" {
		return "", false
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", false
	}

	port := parsed.Port()
	if port != "" && port != "80" && port != "443" {
		host = host + ":" + port
	}

	return host, true
}

func isSubdomainOf(host string, seed string) bool {
	return host != seed && strings.HasSuffix(host, "."+seed)
}

func countLines(filePath string) int {
	if !fileExists(filePath) {
		return 0
	}
	out, err := exec.Command("sh", "-c", fmt.Sprintf("wc -l < %s", shellEscape(filePath))).Output()
	if err != nil {
		return 0
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return count
}

func writeFileAtomic(filePath string, data []byte) error {
	tempFile := filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempFile, filePath)
}

func resolveResumeStatePath(baseFolder string) string {
	return filepath.Join(baseFolder, resumeStateFileName)
}

func loadResumeState(filePath string) (*resumeState, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var state resumeState
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, err
	}
	if state.Completed == nil {
		state.Completed = []string{}
	}

	return &state, nil
}

func saveResumeState(filePath string, state *resumeState) error {
	state.UpdatedAt = time.Now().Format(time.RFC3339)
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return writeFileAtomic(filePath, content)
}

func completedTargetsSet(completed []string) map[string]bool {
	set := make(map[string]bool, len(completed))
	for _, target := range completed {
		target = strings.TrimSpace(target)
		if target != "" {
			set[target] = true
		}
	}
	return set
}

func normalizePathForState(filePath string) string {
	if filePath == "" {
		return ""
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return filePath
	}
	return absPath
}

func printResumeStatus(completedCount, totalTargets int, statePath string) {
	fmt.Println(color.HiCyanString("┌──────────────────────────────────────────────┐"))
	fmt.Printf("│  %s                            │\n", color.HiWhiteString("RESUME MODE ENABLED"))
	fmt.Println(color.HiCyanString("├──────────────────────────────────────────────┤"))
	fmt.Printf("│  Completed Targets : %-21d │\n", completedCount)
	fmt.Printf("│  Remaining Targets : %-21d │\n", totalTargets-completedCount)
	fmt.Printf("│  State File        : %-21s │\n", filepath.Base(statePath))
	fmt.Println(color.HiCyanString("└──────────────────────────────────────────────┘"))
	fmt.Println("")
}

func runShellCommand(command string, verbose bool) error {
	if verbose {
		color.New(color.FgHiBlack).Printf("[CMD] %s\n", command)
	}
	cmd := exec.Command("sh", "-c", command)
	if verbose {
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// runTool com visual moderno e spinner
func runTool(command, toolName, outputFile string, verbose bool) {
	start := time.Now()
	prevCount := countLines(outputFile)

	// Inicia Spinner
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond) // Estilo "dots"
	s.Suffix = fmt.Sprintf("  Running %s...", colorTool(strings.ToUpper(toolName)))
	s.Color("cyan")
	s.Start()

	// --- Lógica de Execução (Mantida Idêntica) ---
	if toolName == "waymore" {
		tempWaymore := outputFile + ".tmp"
		cmdWithTemp := strings.Replace(command, outputFile, tempWaymore, 1)
		os.Remove(tempWaymore)

		runShellCommand(cmdWithTemp, verbose)

		if fileExists(tempWaymore) {
			runShellCommand(fmt.Sprintf("cat %s >> %s", shellEscape(tempWaymore), shellEscape(outputFile)), verbose)
			os.Remove(tempWaymore)
		}
	} else {
		fullCommand := fmt.Sprintf("%s >> %s", command, shellEscape(outputFile))
		runShellCommand(fullCommand, verbose)
	}

	// Ordenação individual
	sortCmd := fmt.Sprintf("sort -u %s -o %s", shellEscape(outputFile), shellEscape(outputFile))
	runShellCommand(sortCmd, verbose)
	// ---------------------------------------------

	s.Stop() // Para o spinner

	// Estatísticas
	elapsed := time.Since(start).Round(time.Second)
	currentCount := countLines(outputFile)
	newInThisTool := currentCount - prevCount

	// Formatação Visual (Alinhamento em colunas)
	// %-12s = Alinha texto à esquerda com 12 espaços
	// %6s   = Alinha à direita

	toolLabel := fmt.Sprintf("%-12s", strings.ToUpper(toolName))
	timeLabel := fmt.Sprintf("%6s", elapsed)
	totalLabel := fmt.Sprintf("%8d urls", currentCount)

	var newLabel string
	if newInThisTool > 0 {
		newLabel = colorNew(fmt.Sprintf("+%d new", newInThisTool))
	} else {
		newLabel = colorZero("0 new")
	}

	// Output final da linha
	fmt.Printf(" %s %s  %s  %s  %s\n",
		iconCheck,
		colorTool(toolLabel),
		colorTime(timeLabel),
		totalLabel,
		newLabel,
	)
}

func aggregateAndClean(toolFiles map[string]string, urlsFile string, oldGlobalCount int, quiet bool) {
	// Spinner para a agregação
	fmt.Println("")
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = "  Aggregating and deduplicating results..."
	s.Color("yellow")
	s.Start()

	rawCombined := urlsFile + ".tmp"
	os.Remove(rawCombined)

	// Salvar URLs antigas antes de agregar (para calcular diff depois)
	oldURLs := make(map[string]bool)
	if fileExists(urlsFile) {
		oldContent, err := os.ReadFile(urlsFile)
		if err == nil {
			for _, line := range strings.Split(string(oldContent), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					oldURLs[line] = true
				}
			}
		}
	}

	var filesToMerge []string
	if fileExists(urlsFile) {
		filesToMerge = append(filesToMerge, urlsFile)
	}
	for _, f := range toolFiles {
		if fileExists(f) {
			filesToMerge = append(filesToMerge, f)
		}
	}

	if len(filesToMerge) > 0 {
		quotedFiles := make([]string, 0, len(filesToMerge))
		for _, file := range filesToMerge {
			quotedFiles = append(quotedFiles, shellEscape(file))
		}
		cmdCat := fmt.Sprintf("cat %s >> %s", strings.Join(quotedFiles, " "), shellEscape(rawCombined))
		runShellCommand(cmdCat, false) // Agregação interna não precisa de verbose
		cmdSort := fmt.Sprintf("sort -u %s -o %s", shellEscape(rawCombined), shellEscape(urlsFile))
		runShellCommand(cmdSort, false)
		os.Remove(rawCombined)
	}

	s.Stop()

	// Calcular URLs novas (diff entre arquivo final e antigas)
	var newURLs []string
	if fileExists(urlsFile) {
		newContent, err := os.ReadFile(urlsFile)
		if err == nil {
			for _, line := range strings.Split(string(newContent), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !oldURLs[line] {
					newURLs = append(newURLs, line)
				}
			}
		}
	}

	// Ordenar as novas URLs (ascending)
	sort.Strings(newURLs)

	// Salvar last_results.txt
	lastResultsFile := filepath.Join(filepath.Dir(urlsFile), "last_results.txt")
	if len(newURLs) > 0 {
		os.WriteFile(lastResultsFile, []byte(strings.Join(newURLs, "\n")+"\n"), 0644)
	} else {
		// Arquivo vazio se não houver novas URLs
		os.WriteFile(lastResultsFile, []byte{}, 0644)
	}

	// Stats Finais
	newGlobalCount := countLines(urlsFile)
	realNewURLs := len(newURLs)

	// Caixa de Resumo Moderno
	fmt.Println("")
	fmt.Println(color.HiBlackString("┌──────────────────────────────────────────────┐"))
	fmt.Printf("│  %s                 │\n", color.HiWhiteString("FINAL RESULTS SUMMARY"))
	fmt.Println(color.HiBlackString("├──────────────────────────────────────────────┤"))
	fmt.Printf("│  Previous Total     : %-22d │\n", oldGlobalCount)
	fmt.Printf("│  Current Total      : %-22d │\n", newGlobalCount)
	fmt.Println(color.HiBlackString("│                                              │"))

	if realNewURLs > 0 {
		fmt.Printf("│  %s : %-22s │\n", color.HiGreenString("UNIQUE NEW URLS"), colorNew(fmt.Sprintf("+%d", realNewURLs)))
	} else {
		fmt.Printf("│  %s           : %-22s │\n", "Unique New URLs", color.HiBlackString("0"))
	}
	fmt.Println(color.HiBlackString("└──────────────────────────────────────────────┘"))

	// Mostrar as novas URLs no terminal (ordenadas ascending)
	if len(newURLs) > 0 && !quiet {
		fmt.Println("")
		fmt.Println(color.HiCyanString("┌──────────────────────────────────────────────┐"))
		fmt.Printf("│  %s                       │\n", color.HiWhiteString("NEW URLS FOUND"))
		fmt.Println(color.HiCyanString("└──────────────────────────────────────────────┘"))
		for _, url := range newURLs {
			fmt.Printf("  %s %s\n", color.HiGreenString("→"), url)
		}
	}
	fmt.Println("")
}

func extractJSURLs(urlsFile string) (int, int) {
	if !fileExists(urlsFile) {
		return 0, 0
	}

	content, err := os.ReadFile(urlsFile)
	if err != nil {
		return 0, 0
	}

	jsFile := filepath.Join(filepath.Dir(urlsFile), "js.txt")
	jsUniqueFile := filepath.Join(filepath.Dir(urlsFile), "js_unique.txt")
	prevCount := countLines(jsUniqueFile)

	jsSet := make(map[string]bool)
	jsUniqueSet := make(map[string]bool)
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isJSURL(line) {
			jsSet[line] = true
			if normalized, ok := normalizeJSURL(line); ok {
				jsUniqueSet[normalized] = true
			}
		}
	}

	var jsURLs []string
	for jsURL := range jsSet {
		jsURLs = append(jsURLs, jsURL)
	}
	sort.Strings(jsURLs)

	var jsUniqueURLs []string
	for jsURL := range jsUniqueSet {
		jsUniqueURLs = append(jsUniqueURLs, jsURL)
	}
	sort.Strings(jsUniqueURLs)

	if len(jsURLs) > 0 {
		os.WriteFile(jsFile, []byte(strings.Join(jsURLs, "\n")+"\n"), 0644)
	} else {
		os.WriteFile(jsFile, []byte{}, 0644)
	}

	if len(jsUniqueURLs) > 0 {
		os.WriteFile(jsUniqueFile, []byte(strings.Join(jsUniqueURLs, "\n")+"\n"), 0644)
	} else {
		os.WriteFile(jsUniqueFile, []byte{}, 0644)
	}

	currentCount := len(jsUniqueURLs)
	newCount := currentCount - prevCount
	if newCount < 0 {
		newCount = 0
	}

	return currentCount, newCount
}

func extractUniqueURLs(urlsFile string) (int, int) {
	if !fileExists(urlsFile) {
		return 0, 0
	}

	content, err := os.ReadFile(urlsFile)
	if err != nil {
		return 0, 0
	}

	uniqueFile := filepath.Join(filepath.Dir(urlsFile), "urls_unique.txt")
	prevCount := countLines(uniqueFile)

	uniqueSet := make(map[string]bool)
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if normalized, ok := normalizeUniqueURL(line); ok {
			uniqueSet[normalized] = true
		}
	}

	var uniqueURLs []string
	for normalizedURL := range uniqueSet {
		uniqueURLs = append(uniqueURLs, normalizedURL)
	}
	sort.Strings(uniqueURLs)

	if len(uniqueURLs) > 0 {
		os.WriteFile(uniqueFile, []byte(strings.Join(uniqueURLs, "\n")+"\n"), 0644)
	} else {
		os.WriteFile(uniqueFile, []byte{}, 0644)
	}

	currentCount := len(uniqueURLs)
	newCount := currentCount - prevCount
	if newCount < 0 {
		newCount = 0
	}

	return currentCount, newCount
}

func extractSubdomains(urlsFile string, knownTargets []string) (int, int) {
	if !fileExists(urlsFile) {
		return 0, 0
	}

	content, err := os.ReadFile(urlsFile)
	if err != nil {
		return 0, 0
	}

	subdomainsFile := filepath.Join(filepath.Dir(urlsFile), "subdomains.txt")
	subdomainsNewFile := filepath.Join(filepath.Dir(urlsFile), "subdomains_new.txt")
	prevCount := countLines(subdomainsNewFile)

	knownTargetsSet := make(map[string]bool)
	for _, target := range knownTargets {
		if normalized, ok := normalizeHost(target); ok {
			knownTargetsSet[normalized] = true
		}
	}

	subdomainsSet := make(map[string]bool)
	subdomainsNewSet := make(map[string]bool)
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if host, ok := normalizeHost(line); ok {
			for seed := range knownTargetsSet {
				if isSubdomainOf(host, seed) {
					subdomainsSet[host] = true
					if !knownTargetsSet[host] {
						subdomainsNewSet[host] = true
					}
					break
				}
			}
		}
	}

	var subdomains []string
	for host := range subdomainsSet {
		subdomains = append(subdomains, host)
	}
	sort.Strings(subdomains)

	var subdomainsNew []string
	for host := range subdomainsNewSet {
		subdomainsNew = append(subdomainsNew, host)
	}
	sort.Strings(subdomainsNew)

	if len(subdomains) > 0 {
		os.WriteFile(subdomainsFile, []byte(strings.Join(subdomains, "\n")+"\n"), 0644)
	} else {
		os.WriteFile(subdomainsFile, []byte{}, 0644)
	}

	if len(subdomainsNew) > 0 {
		os.WriteFile(subdomainsNewFile, []byte(strings.Join(subdomainsNew, "\n")+"\n"), 0644)
	} else {
		os.WriteFile(subdomainsNewFile, []byte{}, 0644)
	}

	currentCount := len(subdomainsNew)
	newCount := currentCount - prevCount
	if newCount < 0 {
		newCount = 0
	}

	return len(subdomains), newCount
}

func sanitizeTargetName(target string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		" ", "_",
		"\t", "_",
	)
	sanitized := strings.TrimSpace(replacer.Replace(target))
	sanitized = strings.Trim(sanitized, "._-")
	if sanitized == "" {
		return "target"
	}
	return sanitized
}

func loadTargetsFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var targets []string
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !seen[line] {
			targets = append(targets, line)
			seen[line] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return targets, nil
}

func buildToolFiles(endpointsDir string) map[string]string {
	return map[string]string{
		"waymore":     filepath.Join(endpointsDir, "waymore.txt"),
		"waybackurls": filepath.Join(endpointsDir, "waybackurls.txt"),
		"gau":         filepath.Join(endpointsDir, "gau.txt"),
		"gau_subs":    filepath.Join(endpointsDir, "gau_subs.txt"),
		"xurlfind3r":  filepath.Join(endpointsDir, "xurlfind3r.txt"),
		"urlscan":     filepath.Join(endpointsDir, "urlscan.txt"),
		"urlfinder":   filepath.Join(endpointsDir, "urlfinder.txt"),
		"ducker":      filepath.Join(endpointsDir, "ducker.txt"),
	}
}

func buildToolCommands(domain string, toolFiles map[string]string) map[string]string {
	return map[string]string{
		"waybackurls": fmt.Sprintf("waybackurls %s", shellEscape(domain)),
		"gau":         fmt.Sprintf("gau %s", shellEscape(domain)),
		"gau_subs":    fmt.Sprintf("gau %s --subs", shellEscape(domain)),
		"xurlfind3r":  fmt.Sprintf("xurlfind3r -d %s --include-subdomains -s", shellEscape(domain)),
		"urlscan": fmt.Sprintf(`curl -s "https://urlscan.io/api/v1/search/?q=page.domain:%s&size=10000" -H "API-Key: %s" | jq -r '.results[].page.url'`,
			domain, os.Getenv("URLSCAN")),
		"urlfinder": fmt.Sprintf("urlfinder -d %s -all", shellEscape(domain)),
		"ducker":    fmt.Sprintf("ducker -q %s", shellEscape("site:"+domain)),
		"waymore":   fmt.Sprintf("waymore -i %s -mode U -oU %s", shellEscape(domain), shellEscape(toolFiles["waymore"])),
	}
}

func selectTools(toolsArg string) []string {
	if toolsArg == "" {
		selected := make([]string, len(defaultToolsOrder))
		copy(selected, defaultToolsOrder)
		return selected
	}

	return strings.Split(toolsArg, ",")
}

func discovery(domain, folderName string, toolsArg string, verbose bool, quiet bool, extractJS bool, extractUnique bool, extractSubdomainsEnabled bool, knownTargets []string, currentTarget int, totalTargets int) {
	baseDir := folderName
	endpointsDir := filepath.Join(baseDir, "endpoints")
	os.MkdirAll(endpointsDir, 0755)
	urlsFile := filepath.Join(endpointsDir, "urls.txt")
	oldGlobalCount := countLines(urlsFile)

	printHeader(domain, folderName, currentTarget, totalTargets)

	toolFiles := buildToolFiles(endpointsDir)
	toolCommands := buildToolCommands(domain, toolFiles)
	selectedTools := selectTools(toolsArg)

	sem := make(chan struct{}, MaxConcurrentTools)
	var wg sync.WaitGroup

	for _, tool := range selectedTools {
		tool = strings.TrimSpace(tool)
		cmdStr, exists := toolCommands[tool]
		if !exists && tool != "waymore" {
			continue
		}

		wg.Add(1)
		go func(t, c string) {
			defer wg.Done()
			sem <- struct{}{}
			runTool(c, t, toolFiles[t], verbose)
			<-sem
		}(tool, cmdStr)
	}
	wg.Wait()

	aggregateAndClean(toolFiles, urlsFile, oldGlobalCount, quiet)
	if extractUnique {
		printSectionBox("URL NORMALIZATION")
		uniqueCount, newUniqueCount := extractUniqueURLs(urlsFile)
		uniqueLabel := fmt.Sprintf("%-12s", "URLS UNIQUE")
		totalLabel := fmt.Sprintf("%8d urls", uniqueCount)

		var newLabel string
		if newUniqueCount > 0 {
			newLabel = colorNew(fmt.Sprintf("+%d new", newUniqueCount))
		} else {
			newLabel = colorZero("0 new")
		}

		fmt.Printf(" %s %s  %s  %s\n",
			iconCheck,
			colorTool(uniqueLabel),
			totalLabel,
			newLabel,
		)
		fmt.Println("")
	}
	if extractSubdomainsEnabled {
		printSectionBox("SUBDOMAIN EXTRACTION")
		subdomainsCount, newSubdomainsCount := extractSubdomains(urlsFile, knownTargets)
		subdomainsLabel := fmt.Sprintf("%-12s", "SUBDOMAINS")
		totalLabel := fmt.Sprintf("%8d subs", subdomainsCount)

		var newLabel string
		if newSubdomainsCount > 0 {
			newLabel = colorNew(fmt.Sprintf("+%d new", newSubdomainsCount))
		} else {
			newLabel = colorZero("0 new")
		}

		fmt.Printf(" %s %s  %s  %s\n",
			iconCheck,
			colorTool(subdomainsLabel),
			totalLabel,
			newLabel,
		)
		fmt.Println("")
	}
	if extractJS {
		printSectionBox("JAVASCRIPT EXTRACTION")
		jsCount, newJSCount := extractJSURLs(urlsFile)
		jsLabel := fmt.Sprintf("%-12s", "JS UNIQUE")
		totalLabel := fmt.Sprintf("%8d urls", jsCount)

		var newLabel string
		if newJSCount > 0 {
			newLabel = colorNew(fmt.Sprintf("+%d new", newJSCount))
		} else {
			newLabel = colorZero("0 new")
		}

		fmt.Printf(" %s %s  %s  %s\n",
			iconCheck,
			colorTool(jsLabel),
			totalLabel,
			newLabel,
		)
		fmt.Println("")
	}
}

func runDiscovery(targets []string, baseFolder, toolsArg string, verbose bool, quiet bool, extractJS bool, extractUnique bool, extractSubdomainsEnabled bool, splitPerTarget bool, resumeEnabled bool, restartEnabled bool, listFilePath string) error {
	usedFolders := make(map[string]int)
	totalStart := time.Now()
	resumeStatePath := resolveResumeStatePath(baseFolder)
	var state *resumeState
	completedSet := make(map[string]bool)

	if err := os.MkdirAll(baseFolder, 0755); err != nil {
		return fmt.Errorf("error creating output folder: %w", err)
	}

	if restartEnabled {
		state = &resumeState{
			ListFile:     listFilePath,
			OutputFolder: normalizePathForState(baseFolder),
			Completed:    []string{},
		}
		if err := saveResumeState(resumeStatePath, state); err != nil {
			return fmt.Errorf("error resetting resume state: %w", err)
		}
		if !quiet {
			color.Yellow("  Restart mode enabled, resetting resume state at %s.", resumeStatePath)
			fmt.Println("")
		}
	}

	if resumeEnabled {
		if fileExists(resumeStatePath) {
			loadedState, err := loadResumeState(resumeStatePath)
			if err != nil {
				return fmt.Errorf("error reading resume state: %w", err)
			}
			if loadedState.ListFile != "" && listFilePath != "" && loadedState.ListFile != listFilePath {
				return fmt.Errorf("resume state belongs to a different target list: %s", loadedState.ListFile)
			}
			state = loadedState
			completedSet = completedTargetsSet(state.Completed)
		} else {
			if !quiet {
				color.Yellow("  Resume file not found at %s, starting fresh.", resumeStatePath)
				fmt.Println("")
			}
			state = &resumeState{}
		}

		if state.ListFile == "" {
			state.ListFile = listFilePath
		}
		if state.OutputFolder == "" {
			state.OutputFolder = normalizePathForState(baseFolder)
		}
		if err := saveResumeState(resumeStatePath, state); err != nil {
			return fmt.Errorf("error initializing resume state: %w", err)
		}

		if !quiet {
			printResumeStatus(len(completedSet), len(targets), resumeStatePath)
		}
	}

	for index, target := range targets {
		if resumeEnabled && completedSet[target] {
			if !quiet {
				fmt.Printf(" %s %s\n", color.HiBlackString("↷"), color.HiBlackString("Skipping completed target: "+target))
			}
			continue
		}

		targetStart := time.Now()
		outputFolder := baseFolder
		if splitPerTarget {
			baseName := sanitizeTargetName(target)
			folderName := baseName
			if usedFolders[baseName] > 0 {
				folderName = fmt.Sprintf("%s_%d", baseName, usedFolders[baseName]+1)
			}
			usedFolders[baseName]++
			outputFolder = filepath.Join(baseFolder, folderName)
		}

		if index > 0 {
			printTargetSeparator()
		}
		currentTarget := 0
		totalTargets := 0
		if len(targets) > 1 {
			currentTarget = index + 1
			totalTargets = len(targets)
		}
		discovery(target, outputFolder, toolsArg, verbose, quiet, extractJS, extractUnique, extractSubdomainsEnabled, targets, currentTarget, totalTargets)

		if resumeEnabled {
			if !completedSet[target] {
				state.Completed = append(state.Completed, target)
				completedSet[target] = true
			}
			if err := saveResumeState(resumeStatePath, state); err != nil {
				return fmt.Errorf("error saving resume state: %w", err)
			}
		}

		if len(targets) > 1 {
			printElapsedBox(fmt.Sprintf("TARGET COMPLETED [%d/%d]", currentTarget, totalTargets), time.Since(targetStart))
		}
	}

	if len(targets) > 1 {
		printElapsedBox("TOTAL EXECUTION TIME", time.Since(totalStart))
	}

	return nil
}

func init() {
	// Garante que ~/go/bin esteja no PATH para ferramentas instaladas via go install
	home, err := os.UserHomeDir()
	if err == nil {
		goBin := filepath.Join(home, "go", "bin")
		path := os.Getenv("PATH")
		if !strings.Contains(path, goBin) {
			os.Setenv("PATH", goBin+string(os.PathListSeparator)+path)
		}
	}
}

func main() {
	domain := flag.String("d", "", "Target domain")
	domainLong := flag.String("domain", "", "Target domain")
	listFile := flag.String("l", "", "File with targets, one per line")
	listFileLong := flag.String("list", "", "File with targets, one per line")
	folderName := flag.String("f", "", "Output folder")
	folderNameLong := flag.String("output", "", "Output folder")
	mergeTargets := flag.Bool("merge-targets", false, "Merge all targets from -l into the same output folder")
	mergeTargetsShort := flag.Bool("m", false, "Merge all targets from -l into the same output folder")
	toolsArg := flag.String("t", "", "Tools list")
	toolsArgLong := flag.String("tools", "", "Tools list")
	extractSubdomains := flag.Bool("extract-subdomains", false, "Extract subdomains into subdomains.txt and newly discovered ones into subdomains_new.txt")
	extractSubdomainsShort := flag.Bool("s", false, "Extract subdomains into subdomains.txt and newly discovered ones into subdomains_new.txt")
	extractUnique := flag.Bool("extract-normalized-urls", false, "Extract normalized unique URLs into urls_unique.txt")
	extractUniqueShort := flag.Bool("u", false, "Extract normalized unique URLs into urls_unique.txt")
	extractJS := flag.Bool("extract-js", false, "Extract JavaScript URLs into js.txt and js_unique.txt")
	extractJSShort := flag.Bool("j", false, "Extract JavaScript URLs into js.txt and js_unique.txt")
	resume := flag.Bool("resume", false, "Resume a target list run from the output folder state file")
	resumeShort := flag.Bool("r", false, "Resume a target list run from the output folder state file")
	restart := flag.Bool("restart", false, "Restart a target list run and reset the saved resume state")
	restartShort := flag.Bool("R", false, "Restart a target list run and reset the saved resume state")
	quiet := flag.Bool("quiet", false, "Quiet mode")
	quietShort := flag.Bool("q", false, "Quiet mode")
	verbose := flag.Bool("v", false, "Verbose mode")
	verboseLong := flag.Bool("verbose", false, "Verbose mode")
	flag.Parse()

	domainValue := *domain
	if domainValue == "" {
		domainValue = *domainLong
	}
	listFileValue := *listFile
	if listFileValue == "" {
		listFileValue = *listFileLong
	}
	folderNameValue := *folderName
	if folderNameValue == "" {
		folderNameValue = *folderNameLong
	}
	toolsArgValue := *toolsArg
	if toolsArgValue == "" {
		toolsArgValue = *toolsArgLong
	}
	mergeTargetsEnabled := *mergeTargets || *mergeTargetsShort
	extractSubdomainsEnabled := *extractSubdomains || *extractSubdomainsShort
	extractUniqueEnabled := *extractUnique || *extractUniqueShort
	extractJSEnabled := *extractJS || *extractJSShort
	resumeEnabled := *resume || *resumeShort
	restartEnabled := *restart || *restartShort
	quietEnabled := *quiet || *quietShort
	verboseEnabled := *verbose || *verboseLong

	if resumeEnabled && restartEnabled {
		color.Red("  ✖ Error: use either -r/--resume or -R/--restart, not both.")
		os.Exit(1)
	}

	if folderNameValue == "" || (domainValue == "" && listFileValue == "") || (domainValue != "" && listFileValue != "") {
		// Mensagem de erro mais bonita
		fmt.Println("")
		color.Red("  ✖ Error: Invalid arguments.")
		fmt.Println("  Usage: ufinder -d domain.com -f output_folder")
		fmt.Println("         ufinder -l targets.txt -f output_folder")
		fmt.Println("")
		os.Exit(1)
	}

	if !quietEnabled {
		printBanner()
	}

	if listFileValue != "" {
		targets, err := loadTargetsFromFile(listFileValue)
		if err != nil {
			color.Red("  ✖ Error reading target list: %v", err)
			os.Exit(1)
		}
		if len(targets) == 0 {
			color.Red("  ✖ Error: target list is empty.")
			os.Exit(1)
		}
		if err := runDiscovery(targets, folderNameValue, toolsArgValue, verboseEnabled, quietEnabled, extractJSEnabled, extractUniqueEnabled, extractSubdomainsEnabled, !mergeTargetsEnabled, resumeEnabled, restartEnabled, normalizePathForState(listFileValue)); err != nil {
			color.Red("  ✖ Error: %v", err)
			os.Exit(1)
		}
		return
	}

	if err := runDiscovery([]string{domainValue}, folderNameValue, toolsArgValue, verboseEnabled, quietEnabled, extractJSEnabled, extractUniqueEnabled, extractSubdomainsEnabled, false, false, false, ""); err != nil {
		color.Red("  ✖ Error: %v", err)
		os.Exit(1)
	}
}
