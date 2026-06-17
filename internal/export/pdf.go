package export

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"

	"requirement-pipeline/internal/domain"
)

var numberedHeadingPattern = regexp.MustCompile(`^\d+[\).]\s+`)

type PDFOptions struct {
	GeneratedAt     time.Time
	MermaidRenderer MermaidRenderer
}

type pdfTheme struct {
	ink       colorRGB
	muted     colorRGB
	navy      colorRGB
	blue      colorRGB
	green     colorRGB
	orange    colorRGB
	purple    colorRGB
	lightBlue colorRGB
	lightGray colorRGB
	line      colorRGB
}

type colorRGB struct {
	r int
	g int
	b int
}

func WriteRefinedRequirementsPDF(outputDir string, audit domain.AuditRun) (string, error) {
	return WriteRefinedRequirementsPDFWithOptions(outputDir, audit, PDFOptions{})
}

func WriteRefinedRequirementsPDFWithOptions(outputDir string, audit domain.AuditRun, options PDFOptions) (string, error) {
	artifact, ok := findArtifact(audit.Artifacts, domain.ArtifactRefinedRequirements)
	if !ok {
		return "", fmt.Errorf("artifact %s not found for run %s", domain.ArtifactRefinedRequirements, audit.Run.ID)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}

	if options.GeneratedAt.IsZero() {
		options.GeneratedAt = time.Now()
	}
	if options.MermaidRenderer == nil {
		options.MermaidRenderer = MermaidCLIRenderer{}
	}
	path := filepath.Join(outputDir, fmt.Sprintf("refined_requirements_%s_%s.pdf", audit.Run.ID, audit.Meeting.ID))
	pdf := gofpdf.New("P", "mm", "A4", "")
	translate := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetTitle(translate("Documento de Requisitos"), false)
	pdf.SetAuthor("Requirement Pipeline", false)
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	theme := defaultTheme()

	addPortraitPage(pdf)
	writeCover(pdf, translate, theme, audit, options.GeneratedAt)
	writeRequirementsGuide(pdf, translate, theme)

	addPortraitPage(pdf)
	writeBusinessDiagrams(pdf, translate, theme, audit.Artifacts, options.MermaidRenderer)

	addPortraitPage(pdf)
	writeRefinedRequirements(pdf, translate, theme, artifact.Content)

	if err := pdf.OutputFileAndClose(path); err != nil {
		return "", err
	}
	return path, nil
}

func writeBusinessDiagrams(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, artifacts []domain.Artifact, renderer MermaidRenderer) {
	setText(pdf, theme.ink)
	sectionTitle(pdf, translate, theme, "Fluxos Do Negócio Identificados")
	writeLead(pdf, translate, theme, "Os diagramas abaixo são gerados a partir dos requisitos refinados e representam fluxos, jornadas, estados ou decisões específicas do negócio discutido na reunião.")

	artifact, ok := findArtifact(artifacts, domain.ArtifactBusinessDiagrams)
	if !ok {
		writeDiagramFallback(pdf, translate, theme, "Diagramas de negócio indisponíveis", "Esta execução não possui artifact business_diagrams. O PDF foi gerado com os requisitos textuais para manter compatibilidade com execuções anteriores.")
		return
	}

	document, err := ParseBusinessDiagrams(artifact.Content)
	if err != nil {
		writeDiagramFallback(pdf, translate, theme, "Diagramas de negócio inválidos", fmt.Sprintf("O artifact business_diagrams não pôde ser interpretado: %v", err))
		return
	}

	for i, diagram := range document.Diagrams {
		rendered, err := renderer.RenderMermaid(diagram.Mermaid)
		if err != nil {
			writeDiagramFallback(pdf, translate, theme, diagram.Title, fmt.Sprintf("Não foi possível renderizar este diagrama Mermaid: %v", err))
			continue
		}
		if i > 0 || shouldUseLandscape(rendered) {
			if shouldUseLandscape(rendered) {
				addLandscapePage(pdf)
			} else {
				addPortraitPage(pdf)
			}
		}
		writeRenderedDiagram(pdf, translate, theme, diagram, rendered)
	}
}

func writeRenderedDiagram(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, diagram BusinessDiagram, rendered RenderedDiagram) {
	sectionTitle(pdf, translate, theme, diagram.Title)
	if strings.TrimSpace(diagram.Description) != "" {
		writeLead(pdf, translate, theme, diagram.Description)
		pdf.Ln(4)
	}

	pageW, pageH := pdf.GetPageSize()
	left, top, right, bottom := pdf.GetMargins()
	maxW := pageW - left - right
	maxH := pageH - bottom - pdf.GetY()
	if maxH < 80 {
		if shouldUseLandscape(rendered) {
			addLandscapePage(pdf)
		} else {
			addPortraitPage(pdf)
		}
		sectionTitle(pdf, translate, theme, diagram.Title)
		pageW, pageH = pdf.GetPageSize()
		left, top, right, bottom = pdf.GetMargins()
		maxW = pageW - left - right
		maxH = pageH - top - bottom - 20
	}

	imageW, imageH := fitImage(float64(rendered.WidthPx), float64(rendered.HeightPx), maxW, maxH)
	x := left + (maxW-imageW)/2
	y := pdf.GetY()
	pdf.ImageOptions(rendered.Path, x, y, imageW, imageH, false, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, 0, "")
	pdf.SetY(y + imageH + 6)
}

func fitImage(width float64, height float64, maxWidth float64, maxHeight float64) (float64, float64) {
	if width <= 0 || height <= 0 {
		return maxWidth, maxHeight
	}
	ratio := width / height
	imageW := maxWidth
	imageH := imageW / ratio
	if imageH > maxHeight {
		imageH = maxHeight
		imageW = imageH * ratio
	}
	return imageW, imageH
}

func shouldUseLandscape(rendered RenderedDiagram) bool {
	return rendered.WidthPx > rendered.HeightPx && rendered.WidthPx >= 900
}

func writeDiagramFallback(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, title string, message string) {
	ensureSpace(pdf, 34)
	pdf.Ln(6)
	y := pdf.GetY()
	setFill(pdf, theme.lightGray)
	setDraw(pdf, theme.line)
	pdf.RoundedRect(18, y, 174, 32, 2, "1234", "FD")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(24, y+6)
	pdf.CellFormat(162, 5, translate(title), "", 1, "L", false, 0, "")
	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "", 8.5)
	pdf.SetX(24)
	pdf.MultiCell(162, 4.5, translate(message), "", "L", false)
	pdf.SetY(y + 38)
}

func defaultTheme() pdfTheme {
	return pdfTheme{
		ink:       colorRGB{33, 37, 41},
		muted:     colorRGB{92, 99, 112},
		navy:      colorRGB{31, 45, 61},
		blue:      colorRGB{67, 97, 238},
		green:     colorRGB{42, 157, 143},
		orange:    colorRGB{244, 162, 97},
		purple:    colorRGB{114, 90, 193},
		lightBlue: colorRGB{239, 244, 255},
		lightGray: colorRGB{248, 249, 250},
		line:      colorRGB{218, 224, 232},
	}
}

func addPortraitPage(pdf *gofpdf.Fpdf) {
	pdf.AddPageFormat("P", gofpdf.SizeType{Wd: 210, Ht: 297})
	pdf.SetMargins(18, 18, 18)
}

func addLandscapePage(pdf *gofpdf.Fpdf) {
	pdf.AddPageFormat("L", gofpdf.SizeType{Wd: 297, Ht: 210})
	pdf.SetMargins(12, 12, 12)
}

func writeCover(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, audit domain.AuditRun, generatedAt time.Time) {
	setFill(pdf, theme.navy)
	pdf.Rect(0, 0, 210, 58, "F")
	setText(pdf, colorRGB{255, 255, 255})
	pdf.SetFont("Arial", "B", 24)
	pdf.SetXY(18, 18)
	pdf.CellFormat(0, 10, translate("Documento de Requisitos"), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.SetX(18)
	pdf.MultiCell(150, 6, translate("Especificação consolidada para apoiar Product Owner, análise, desenvolvimento e validação do produto."), "", "L", false)

	setText(pdf, theme.ink)
	pdf.SetY(76)
	infoCard(pdf, translate, theme, 18, pdf.GetY(), 174, []labelValue{
		{"Reunião", audit.Meeting.Title},
		{"Idioma", audit.Meeting.Language},
		{"Meeting ID", audit.Meeting.ID},
		{"Run ID", audit.Run.ID},
		{"Gerado em", generatedAt.Format(time.RFC3339)},
	})
	pdf.SetY(136)
	sectionTitle(pdf, translate, theme, "Como usar este documento")
	writeLead(pdf, translate, theme, "Este documento deve ser lido como entrada para planejamento e implementação. Primeiro identifique as capacidades funcionais, depois aplique as regras de negócio, valide os requisitos não funcionais e resolva as decisões pendentes antes de fechar escopo técnico.")
}

func writeRequirementsGuide(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme) {
	pdf.Ln(8)
	sectionTitle(pdf, translate, theme, "Guia Para Implementação")
	writeLead(pdf, translate, theme, "A leitura recomendada para o time é por tipo de requisito. Isso ajuda a separar o que deve ser construído, quais regras controlam o comportamento, quais restrições precisam ser respeitadas e quais pontos ainda exigem decisão do Product Owner.")
	pdf.Ln(5)

	x := 18.0
	y := pdf.GetY()
	drawRequirementCard(pdf, translate, theme, x, y, 82, "RF", "Requisitos Funcionais", "Funcionalidades, fluxos, telas, integrações e ações que o sistema deve entregar.", theme.blue)
	drawRequirementCard(pdf, translate, theme, x+92, y, 82, "RN", "Regras de Negócio", "Políticas, limites, cálculos, estados e decisões que controlam as funcionalidades.", theme.green)
	drawRequirementCard(pdf, translate, theme, x, y+38, 82, "RNF", "Requisitos Não Funcionais", "Critérios de qualidade: desempenho, segurança, disponibilidade, acessibilidade e operação.", theme.orange)
	drawRequirementCard(pdf, translate, theme, x+92, y+38, 82, "R", "Restrições", "Limites legais, técnicos, fiscais, regulatórios e escolhas que restringem a solução.", theme.purple)
	pdf.SetY(y + 84)

	sectionTitle(pdf, translate, theme, "Checklist Para O Time")
	items := []string{
		"Transformar cada RF em épicos, histórias ou tarefas implementáveis.",
		"Associar cada RN às funcionalidades impactadas e criar testes de regra.",
		"Validar RNF com critérios mensuráveis sempre que possível.",
		"Marcar decisões pendentes como riscos de escopo antes da implementação.",
		"Usar critérios de aceite como base para testes funcionais e homologação.",
	}
	for _, item := range items {
		writeChecklistItem(pdf, translate, theme, item)
	}
}

func writeBusinessImplementationFlow(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme) {
	setText(pdf, theme.ink)
	pdf.SetY(18)
	sectionTitle(pdf, translate, theme, "Fluxograma Para Implementação Do Produto")
	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "", 9.3)
	pdf.MultiCell(0, 5, translate("Este fluxo é genérico para qualquer sistema descrito neste documento. Ele mostra como o time deve sair da ideia ou reunião inicial, organizar os requisitos e transformar o material em backlog implementável."), "", "L", false)

	nodeW := 72.0
	nodeH := 24.0
	xLeft := 22.0
	xRight := 116.0
	y1 := 58.0
	y2 := 96.0
	y3 := 134.0
	y4 := 174.0
	y5 := 212.0

	drawBusinessNode(pdf, translate, theme, xLeft, y1, nodeW, nodeH, "1", "Ideia ou reunião", "Problema, contexto e objetivo", theme.blue)
	drawBusinessNode(pdf, translate, theme, xRight, y1, nodeW, nodeH, "2", "Atores e jornadas", "Usuários, papéis e fluxos", theme.purple)
	drawBusinessNode(pdf, translate, theme, xLeft, y2, nodeW, nodeH, "3", "Capacidades", "RFs e funcionalidades", theme.green)
	drawBusinessNode(pdf, translate, theme, xRight, y2, nodeW, nodeH, "4", "Regras e qualidade", "RNs, RNFs e restrições", theme.orange)
	drawDecisionNode(pdf, translate, theme, xLeft, y3, nodeW, nodeH, "Há decisão pendente?", theme.purple)
	drawBusinessNode(pdf, translate, theme, xRight, y3, nodeW, nodeH, "5", "Voltar ao PO", "Resolver dúvidas e escopo", theme.orange)
	drawBusinessNode(pdf, translate, theme, xLeft, y4, nodeW, nodeH, "6", "Priorizar backlog", "Épicos, histórias e releases", theme.blue)
	drawBusinessNode(pdf, translate, theme, xRight, y4, nodeW, nodeH, "7", "Implementar", "Código, integrações e dados", theme.green)
	drawBusinessNode(pdf, translate, theme, xLeft, y5, nodeW, nodeH, "8", "Validar", "Testes e homologação", theme.purple)
	drawBusinessNode(pdf, translate, theme, xRight, y5, nodeW, nodeH, "9", "Evoluir", "Feedback e novas versões", theme.blue)

	drawArrow(pdf, xLeft+nodeW, y1+nodeH/2, xRight-4, y1+nodeH/2, theme.muted)
	drawArrow(pdf, xRight+nodeW/2, y1+nodeH, xRight+nodeW/2, y2-4, theme.muted)
	drawArrow(pdf, xRight, y2+nodeH/2, xLeft+nodeW+4, y2+nodeH/2, theme.muted)
	drawArrow(pdf, xLeft+nodeW/2, y2+nodeH, xLeft+nodeW/2, y3-4, theme.muted)
	drawArrow(pdf, xLeft+nodeW, y3+nodeH/2, xRight-4, y3+nodeH/2, theme.muted)
	drawArrow(pdf, xLeft+nodeW/2, y3+nodeH, xLeft+nodeW/2, y4-4, theme.muted)
	drawArrow(pdf, xLeft+nodeW, y4+nodeH/2, xRight-4, y4+nodeH/2, theme.muted)
	drawArrow(pdf, xRight, y5+nodeH/2, xLeft+nodeW+4, y5+nodeH/2, theme.muted)
	drawArrow(pdf, xRight+nodeW/2, y4+nodeH, xRight+nodeW/2, y5-4, theme.muted)

	setText(pdf, theme.orange)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetXY(xLeft+nodeW+6, y3+2)
	pdf.CellFormat(12, 4, translate("SIM"), "", 0, "L", false, 0, "")
	setText(pdf, theme.green)
	pdf.SetXY(xLeft+nodeW/2+3, y3+31)
	pdf.CellFormat(12, 4, translate("NÃO"), "", 0, "L", false, 0, "")

	pdf.SetY(248)
	drawImplementationNotes(pdf, translate, theme)
}

func writeExecutiveSummary(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, audit domain.AuditRun) {
	pdf.Ln(8)
	sectionTitle(pdf, translate, theme, "Resumo Executivo")
	y := pdf.GetY()
	summaryMetric(pdf, translate, theme, 18, y, 39, "Status", string(audit.Run.Status), theme.green)
	summaryMetric(pdf, translate, theme, 62, y, 39, "Etapas", fmt.Sprintf("%d", len(audit.Stages)), theme.blue)
	summaryMetric(pdf, translate, theme, 106, y, 39, "Artefatos", fmt.Sprintf("%d", len(audit.Artifacts)), theme.purple)
	summaryMetric(pdf, translate, theme, 150, y, 42, "Prompts", fmt.Sprintf("%d", len(audit.Prompts)), theme.orange)
	pdf.SetY(y + 34)

	writeLead(pdf, translate, theme, "A pipeline transforma áudio em requisitos por meio de etapas sequenciais. Cada etapa registra modelo, duração, entrada, saída e erro quando houver. Isso permite revisar o que foi produzido, onde foi produzido e com qual contexto.")
}

func writePipelineMap(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme) {
	setText(pdf, theme.ink)
	pdf.SetY(14)
	sectionTitle(pdf, translate, theme, "Mapa Visual Da Pipeline")
	pdf.SetFont("Arial", "", 10)
	setText(pdf, theme.muted)
	pdf.MultiCell(0, 5, translate("Leitura da esquerda para a direita: cada card representa uma transformação do artefato anterior até chegar aos documentos finais."), "", "L", false)

	steps := []struct {
		number string
		title  string
		input  string
		output string
		color  colorRGB
	}{
		{"1", "Transcrição", "Áudio", "transcript", theme.blue},
		{"2", "Extração", "transcript", "requirement_draft", theme.purple},
		{"3", "Revisão", "requirement_draft", "reviewed_requirements", theme.green},
		{"4", "Lacunas", "reviewed_requirements", "gap_analysis", theme.orange},
		{"5", "Refinamento", "review + gaps", "refined_requirements", theme.blue},
		{"6", "Documentação", "refined_requirements", "SRS + histórias + casos", theme.purple},
	}

	x := 12.0
	y := 58.0
	cardW := 41.5
	cardH := 70.0
	gap := 5.0
	for i, step := range steps {
		drawStageCard(pdf, translate, theme, x, y, cardW, cardH, step.number, step.title, step.input, step.output, step.color)
		if i < len(steps)-1 {
			drawArrow(pdf, x+cardW+0.8, y+cardH/2, x+cardW+gap-0.8, y+cardH/2, theme.muted)
		}
		x += cardW + gap
	}

	pdf.SetY(146)
	drawLegendBox(pdf, translate, theme, 18, pdf.GetY(), 261)
}

func writeAuditMap(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme) {
	sectionTitle(pdf, translate, theme, "Rastreabilidade No MongoDB")
	writeLead(pdf, translate, theme, "Além do PDF final, a execução fica auditável no MongoDB. As coleções se conectam por IDs e permitem reconstruir a reunião, cada etapa, seus artefatos, prompts, modelos e métricas.")

	y := pdf.GetY() + 4
	drawCollectionBox(pdf, translate, theme, 18, y, 42, "meetings", "Áudio, título e idioma", theme.blue)
	drawCollectionBox(pdf, translate, theme, 74, y, 42, "pipeline_runs", "Status da execução", theme.green)
	drawCollectionBox(pdf, translate, theme, 130, y, 42, "stages", "Etapas e métricas", theme.orange)
	drawCollectionBox(pdf, translate, theme, 74, y+42, 42, "prompts", "Templates v1", theme.purple)
	drawCollectionBox(pdf, translate, theme, 130, y+42, 42, "artifacts", "Conteúdo gerado", theme.blue)

	drawArrow(pdf, 60, y+14, 74, y+14, theme.muted)
	drawArrow(pdf, 116, y+14, 130, y+14, theme.muted)
	drawArrow(pdf, 151, y+30, 151, y+42, theme.muted)
	drawArrow(pdf, 116, y+56, 130, y+56, theme.muted)

	pdf.SetY(y + 86)
}

func writeExecutionTimeline(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, audit domain.AuditRun) {
	if len(audit.Stages) == 0 {
		return
	}
	sectionTitle(pdf, translate, theme, "Linha Do Tempo Da Execução")
	writeLead(pdf, translate, theme, "Esta visão resume o que aconteceu na execução salva no MongoDB.")

	stages := append([]domain.StageExecution(nil), audit.Stages...)
	slices.SortFunc(stages, func(a, b domain.StageExecution) int { return a.StartedAt.Compare(b.StartedAt) })
	artifactTypesByStage := map[string][]string{}
	for _, artifact := range audit.Artifacts {
		artifactTypesByStage[artifact.StageID] = append(artifactTypesByStage[artifact.StageID], string(artifact.Type))
	}

	pdf.SetFont("Arial", "B", 9)
	setFill(pdf, theme.navy)
	setText(pdf, colorRGB{255, 255, 255})
	pdf.CellFormat(56, 7, translate("Etapa"), "1", 0, "L", true, 0, "")
	pdf.CellFormat(24, 7, translate("Status"), "1", 0, "L", true, 0, "")
	pdf.CellFormat(24, 7, translate("Duração"), "1", 0, "L", true, 0, "")
	pdf.CellFormat(70, 7, translate("Saídas"), "1", 1, "L", true, 0, "")

	pdf.SetFont("Arial", "", 8)
	setText(pdf, theme.ink)
	for i, stage := range stages {
		ensureSpace(pdf, 9)
		fill := i%2 == 0
		if fill {
			setFill(pdf, theme.lightGray)
		} else {
			setFill(pdf, colorRGB{255, 255, 255})
		}
		outputs := strings.Join(artifactTypesByStage[stage.ID], ", ")
		if outputs == "" {
			outputs = "-"
		}
		pdf.CellFormat(56, 7, translate(string(stage.Name)), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(24, 7, translate(string(stage.Status)), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(24, 7, translate(fmt.Sprintf("%d ms", stage.DurationMillis)), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(70, 7, translate(outputs), "1", 1, "L", fill, 0, "")
	}
	pdf.Ln(8)
}

func writeRefinedRequirements(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, content string) {
	sectionTitle(pdf, translate, theme, "Requisitos Refinados")
	writeLead(pdf, translate, theme, "Abaixo está a especificação refinada para implementação. Títulos, grupos e listas foram formatados para facilitar leitura, quebra em backlog, revisão de escopo e validação com stakeholders.")

	for line := range strings.SplitSeq(content, "\n") {
		writeRequirementLine(pdf, translate, theme, line)
	}
}

func writeRequirementLine(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		pdf.Ln(2.5)
		return
	}

	if numberedHeadingPattern.MatchString(trimmed) || isRequirementHeading(trimmed) {
		ensureSpace(pdf, 18)
		pdf.Ln(2)
		setFill(pdf, theme.lightBlue)
		setDraw(pdf, colorRGB{210, 222, 255})
		setText(pdf, theme.navy)
		pdf.SetFont("Arial", "B", 11)
		y := pdf.GetY()
		pdf.RoundedRect(18, y, 174, 10, 2, "1234", "FD")
		pdf.SetXY(22, y+2.2)
		pdf.MultiCell(166, 5, translate(trimmed), "", "L", false)
		pdf.SetY(y + 12)
		return
	}

	if strings.HasPrefix(trimmed, "- ") {
		ensureSpace(pdf, 8)
		setText(pdf, theme.ink)
		pdf.SetFont("Arial", "", 10)
		pdf.SetX(24)
		pdf.CellFormat(4, 5, translate("•"), "", 0, "L", false, 0, "")
		pdf.SetX(29)
		pdf.MultiCell(157, 5.2, translate(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))), "", "L", false)
		return
	}

	ensureSpace(pdf, 8)
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "", 10.2)
	pdf.MultiCell(0, 5.4, translate(trimmed), "", "L", false)
}

func isRequirementHeading(line string) bool {
	prefixes := []string{"RF", "RNF", "RN", "RST", "R0", "Requisitos Funcionais", "Regras de Negócio", "Requisitos Não Funcionais", "Restrições", "Critérios de aceite", "Decisões pendentes", "Backlog", "Roadmap", "Governança"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

type labelValue struct {
	label string
	value string
}

func infoCard(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, x float64, y float64, width float64, rows []labelValue) {
	setFill(pdf, theme.lightGray)
	setDraw(pdf, theme.line)
	pdf.RoundedRect(x, y, width, 46, 2, "1234", "FD")
	pdf.SetY(y + 6)
	for _, row := range rows {
		pdf.SetX(x + 7)
		pdf.SetFont("Arial", "B", 9.5)
		setText(pdf, theme.ink)
		pdf.CellFormat(34, 6, translate(row.label), "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9.5)
		pdf.MultiCell(width-48, 6, translate(row.value), "", "L", false)
	}
}

func summaryMetric(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, x float64, y float64, width float64, label string, value string, accent colorRGB) {
	setFill(pdf, colorRGB{255, 255, 255})
	setDraw(pdf, theme.line)
	pdf.RoundedRect(x, y, width, 26, 2, "1234", "D")
	setFill(pdf, accent)
	pdf.RoundedRect(x, y, width, 5, 2, "12", "F")
	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "", 8)
	pdf.SetXY(x+4, y+8)
	pdf.CellFormat(width-8, 4, translate(label), "", 1, "L", false, 0, "")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 13)
	pdf.SetX(x + 4)
	pdf.CellFormat(width-8, 7, translate(value), "", 1, "L", false, 0, "")
}

func drawStageCard(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, x float64, y float64, w float64, h float64, number string, title string, input string, output string, accent colorRGB) {
	setFill(pdf, colorRGB{255, 255, 255})
	setDraw(pdf, theme.line)
	pdf.RoundedRect(x, y, w, h, 3, "1234", "D")
	setFill(pdf, accent)
	pdf.RoundedRect(x, y, w, 12, 3, "12", "F")
	setText(pdf, colorRGB{255, 255, 255})
	pdf.SetFont("Arial", "B", 11)
	pdf.SetXY(x+3, y+3)
	pdf.CellFormat(w-6, 5, translate(number+". "+title), "", 1, "L", false, 0, "")

	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetXY(x+4, y+18)
	pdf.CellFormat(w-8, 4, translate("ENTRADA"), "", 1, "L", false, 0, "")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "", 8)
	pdf.SetX(x + 4)
	pdf.MultiCell(w-8, 4, translate(input), "", "L", false)

	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetXY(x+4, y+42)
	pdf.CellFormat(w-8, 4, translate("SAÍDA"), "", 1, "L", false, 0, "")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "", 8)
	pdf.SetX(x + 4)
	pdf.MultiCell(w-8, 4, translate(output), "", "L", false)
}

func drawLegendBox(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, x float64, y float64, w float64) {
	setFill(pdf, theme.lightGray)
	setDraw(pdf, theme.line)
	pdf.RoundedRect(x, y, w, 34, 2, "1234", "FD")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(x+6, y+5)
	pdf.CellFormat(w-12, 5, translate("O que acontece em cada etapa?"), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetX(x + 6)
	pdf.MultiCell(w-12, 5, translate("A etapa lê um artefato de entrada, chama o provedor de IA quando necessário, grava métricas/modelo/prompt em stages e persiste o texto gerado em artifacts. O próximo card usa esse resultado como entrada."), "", "L", false)
}

func drawCollectionBox(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, x float64, y float64, w float64, title string, subtitle string, accent colorRGB) {
	setFill(pdf, colorRGB{255, 255, 255})
	setDraw(pdf, theme.line)
	pdf.RoundedRect(x, y, w, 28, 2, "1234", "D")
	setFill(pdf, accent)
	pdf.Rect(x, y, w, 5, "F")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(x+3, y+8)
	pdf.CellFormat(w-6, 5, translate(title), "", 1, "L", false, 0, "")
	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "", 7.5)
	pdf.SetX(x + 3)
	pdf.MultiCell(w-6, 4, translate(subtitle), "", "L", false)
}

func drawRequirementCard(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, x float64, y float64, w float64, code string, title string, description string, accent colorRGB) {
	setFill(pdf, colorRGB{255, 255, 255})
	setDraw(pdf, theme.line)
	pdf.RoundedRect(x, y, w, 30, 2, "1234", "D")
	setFill(pdf, accent)
	pdf.RoundedRect(x, y, 18, 30, 2, "14", "F")
	setText(pdf, colorRGB{255, 255, 255})
	pdf.SetFont("Arial", "B", 11)
	pdf.SetXY(x, y+10)
	pdf.CellFormat(18, 5, translate(code), "", 0, "C", false, 0, "")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 9.5)
	pdf.SetXY(x+22, y+5)
	pdf.CellFormat(w-26, 5, translate(title), "", 1, "L", false, 0, "")
	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "", 8)
	pdf.SetX(x + 22)
	pdf.MultiCell(w-26, 4, translate(description), "", "L", false)
}

func drawBusinessNode(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, x float64, y float64, w float64, h float64, number string, title string, subtitle string, accent colorRGB) {
	setFill(pdf, colorRGB{255, 255, 255})
	setDraw(pdf, theme.line)
	pdf.RoundedRect(x, y, w, h, 2.5, "1234", "D")
	setFill(pdf, accent)
	pdf.Circle(x+6, y+6, 4, "F")
	setText(pdf, colorRGB{255, 255, 255})
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetXY(x+2, y+3.2)
	pdf.CellFormat(8, 4, translate(number), "", 0, "C", false, 0, "")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 8.5)
	pdf.SetXY(x+12, y+4)
	pdf.MultiCell(w-15, 4.2, translate(title), "", "L", false)
	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "", 7)
	pdf.SetXY(x+4, y+13)
	pdf.MultiCell(w-8, 3.6, translate(subtitle), "", "L", false)
}

func drawDecisionNode(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, x float64, y float64, w float64, h float64, title string, accent colorRGB) {
	setFill(pdf, colorRGB{255, 255, 255})
	setDraw(pdf, accent)
	pdf.RoundedRect(x, y, w, h, 3, "1234", "D")
	setFill(pdf, accent)
	pdf.Rect(x, y, w, 5, "F")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 8.5)
	pdf.SetXY(x+4, y+9)
	pdf.MultiCell(w-8, 4.3, translate(title), "", "C", false)
}

func drawImplementationNotes(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme) {
	setFill(pdf, theme.lightGray)
	setDraw(pdf, theme.line)
	pdf.RoundedRect(18, pdf.GetY(), 174, 30, 2, "1234", "FD")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 9.5)
	pdf.SetXY(24, pdf.GetY()+5)
	pdf.CellFormat(0, 5, translate("Pontos de atenção para implementação"), "", 1, "L", false, 0, "")
	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "", 8.5)
	pdf.SetX(24)
	pdf.MultiCell(162, 4.5, translate("Use este fluxo como ponte entre requisitos e execução. Requisitos funcionais viram épicos e histórias; regras de negócio viram critérios e testes; requisitos não funcionais viram metas mensuráveis; decisões pendentes voltam para o PO antes de comprometer desenvolvimento."), "", "L", false)
}

func writeChecklistItem(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, text string) {
	ensureSpace(pdf, 10)
	x := 22.0
	y := pdf.GetY()
	setDraw(pdf, theme.green)
	pdf.Rect(x, y+1, 4, 4, "D")
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(x+8, y)
	pdf.MultiCell(160, 5.2, translate(text), "", "L", false)
}

func drawArrow(pdf *gofpdf.Fpdf, x1 float64, y1 float64, x2 float64, y2 float64, color colorRGB) {
	setDraw(pdf, color)
	pdf.SetLineWidth(0.4)
	pdf.Line(x1, y1, x2, y2)
	if x2 >= x1 {
		pdf.Line(x2, y2, x2-2, y2-1.3)
		pdf.Line(x2, y2, x2-2, y2+1.3)
	} else {
		pdf.Line(x2, y2, x2+2, y2-1.3)
		pdf.Line(x2, y2, x2+2, y2+1.3)
	}
	pdf.SetLineWidth(0.2)
}

func sectionTitle(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, title string) {
	setText(pdf, theme.ink)
	pdf.SetFont("Arial", "B", 15)
	pdf.MultiCell(0, 8, translate(title), "", "L", false)
	setDraw(pdf, theme.navy)
	pdf.Line(pdf.GetX(), pdf.GetY(), pageRight(pdf), pdf.GetY())
	pdf.Ln(5)
}

func writeLead(pdf *gofpdf.Fpdf, translate func(string) string, theme pdfTheme, text string) {
	setText(pdf, theme.muted)
	pdf.SetFont("Arial", "", 10.5)
	pdf.MultiCell(0, 5.8, translate(text), "", "L", false)
}

func ensureSpace(pdf *gofpdf.Fpdf, required float64) {
	_, pageH := pdf.GetPageSize()
	_, _, _, bottom := pdf.GetMargins()
	if pdf.GetY()+required > pageH-bottom {
		addPortraitPage(pdf)
	}
}

func pageRight(pdf *gofpdf.Fpdf) float64 {
	pageW, _ := pdf.GetPageSize()
	_, _, right, _ := pdf.GetMargins()
	return pageW - right
}

func setFill(pdf *gofpdf.Fpdf, color colorRGB) {
	pdf.SetFillColor(color.r, color.g, color.b)
}

func setDraw(pdf *gofpdf.Fpdf, color colorRGB) {
	pdf.SetDrawColor(color.r, color.g, color.b)
}

func setText(pdf *gofpdf.Fpdf, color colorRGB) {
	pdf.SetTextColor(color.r, color.g, color.b)
}

func findArtifact(artifacts []domain.Artifact, artifactType domain.ArtifactType) (domain.Artifact, bool) {
	for _, artifact := range artifacts {
		if artifact.Type == artifactType {
			return artifact, true
		}
	}
	return domain.Artifact{}, false
}
