// ufinder.go
package main

import (
	"bufio"
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

func printHeader(domain, folder string) {
	fmt.Printf("   %s Target: %s\n", iconFire, color.HiWhiteString(domain))
	fmt.Printf("   %s Output: %s\n", iconBox, color.HiWhiteString(folder))
	fmt.Println(strings.Repeat(color.HiBlackString("─"), 60))
	fmt.Println("")
}

func printTargetSeparator() {
	fmt.Println("")
	fmt.Println(color.HiBlackString(strings.Repeat("=", 60)))
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
	prevCount := countLines(jsFile)

	jsSet := make(map[string]bool)
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isJSURL(line) {
			jsSet[line] = true
		}
	}

	var jsURLs []string
	for jsURL := range jsSet {
		jsURLs = append(jsURLs, jsURL)
	}
	sort.Strings(jsURLs)

	if len(jsURLs) > 0 {
		os.WriteFile(jsFile, []byte(strings.Join(jsURLs, "\n")+"\n"), 0644)
	} else {
		os.WriteFile(jsFile, []byte{}, 0644)
	}

	currentCount := len(jsURLs)
	newCount := currentCount - prevCount
	if newCount < 0 {
		newCount = 0
	}

	return currentCount, newCount
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

func discovery(domain, folderName string, toolsArg string, verbose bool, quiet bool, extractJS bool) {
	baseDir := folderName
	endpointsDir := filepath.Join(baseDir, "endpoints")
	os.MkdirAll(endpointsDir, 0755)
	urlsFile := filepath.Join(endpointsDir, "urls.txt")
	oldGlobalCount := countLines(urlsFile)

	printHeader(domain, folderName)

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
	if extractJS {
		fmt.Println(color.HiCyanString("┌──────────────────────────────────────────────┐"))
		fmt.Printf("│  %s                   │\n", color.HiWhiteString("JAVASCRIPT EXTRACTION"))
		fmt.Println(color.HiCyanString("└──────────────────────────────────────────────┘"))
		jsCount, newJSCount := extractJSURLs(urlsFile)
		jsLabel := fmt.Sprintf("%-12s", "JS")
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

func runDiscovery(targets []string, baseFolder, toolsArg string, verbose bool, quiet bool, extractJS bool, splitPerTarget bool) {
	usedFolders := make(map[string]int)

	for index, target := range targets {
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
		discovery(target, outputFolder, toolsArg, verbose, quiet, extractJS)
	}
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
	listFile := flag.String("l", "", "File with targets, one per line")
	folderName := flag.String("f", "", "Output folder")
	mergeTargets := flag.Bool("merge-targets", false, "Merge all targets from -l into the same output folder")
	mergeTargetsShort := flag.Bool("m", false, "Merge all targets from -l into the same output folder")
	toolsArg := flag.String("t", "", "Tools list")
	extractJS := flag.Bool("extract-js", false, "Extract JavaScript URLs into js.txt")
	extractJSShort := flag.Bool("j", false, "Extract JavaScript URLs into js.txt")
	quiet := flag.Bool("quiet", false, "Quiet mode")
	quietShort := flag.Bool("q", false, "Quiet mode")
	verbose := flag.Bool("v", false, "Verbose mode")
	flag.Parse()

	mergeTargetsEnabled := *mergeTargets || *mergeTargetsShort
	extractJSEnabled := *extractJS || *extractJSShort
	quietEnabled := *quiet || *quietShort

	if *folderName == "" || (*domain == "" && *listFile == "") || (*domain != "" && *listFile != "") {
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

	if *listFile != "" {
		targets, err := loadTargetsFromFile(*listFile)
		if err != nil {
			color.Red("  ✖ Error reading target list: %v", err)
			os.Exit(1)
		}
		if len(targets) == 0 {
			color.Red("  ✖ Error: target list is empty.")
			os.Exit(1)
		}
		runDiscovery(targets, *folderName, *toolsArg, *verbose, quietEnabled, extractJSEnabled, !mergeTargetsEnabled)
		return
	}

	runDiscovery([]string{*domain}, *folderName, *toolsArg, *verbose, quietEnabled, extractJSEnabled, false)
}
