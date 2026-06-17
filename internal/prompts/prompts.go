package prompts

import (
	"strings"
	"time"

	"requirement-pipeline/internal/domain"
)

const VersionV1 = "v1"

func Defaults(now time.Time) []domain.Prompt {
	return []domain.Prompt{
		newPrompt(domain.StageRequirementExtraction, `Você é um analista de requisitos experiente.

Idioma da resposta: {{language}}.

Extraia todos os requisitos candidatos da transcrição e classifique-os em:
- Requisitos Funcionais
- Requisitos Não Funcionais
- Regras de Negócio
- Restrições

Transcrição:
{{input}}

Para cada requisito, escreva de forma clara, objetiva e sem duplicidade.`, now),

		newPrompt(domain.StageRequirementReview, `Você é um revisor sênior de requisitos.

Idioma da resposta: {{language}}.

Analise o rascunho abaixo e identifique ambiguidades, duplicidades, inconsistências, conflitos e requisitos mal escritos.

Rascunho:
{{input}}

Estruture a resposta com seções claras, listando cada problema com explicação e sugestão de correção.`, now),

		newPrompt(domain.StageGapAnalysis, `Você é um especialista em elicitação de requisitos.

Idioma da resposta: {{language}}.

Faça uma análise de lacunas nos requisitos revisados:

Requisitos revisados:
{{input}}

Identifique informações faltantes, requisitos implícitos, regras incompletas, perguntas pendentes e possíveis conflitos. Priorize as lacunas mais importantes.`, now),

		newPrompt(domain.StageRequirementRefinement, `Você é um analista de requisitos sênior.

Idioma da resposta: {{language}}.

Refine os requisitos com base nas entradas abaixo:

Requisitos revisados:
{{reviewed_requirements}}

Análise de lacunas:
{{gap_analysis}}

Instruções:
- Elimine ambiguidades e duplicidades
- Complete as lacunas identificadas
- Torne os requisitos claros, mensuráveis e rastreáveis
- Use formato imperativo ("O sistema deve...")`, now),

		newPrompt(domain.StageDiagramGeneration, `Você é um analista de negócios e arquiteto de requisitos.

Idioma da resposta: {{language}}.

Gere diagramas Mermaid específicos para o negócio descrito nos requisitos abaixo.

Regras obrigatórias:
- Responda somente com JSON válido, sem markdown fora do JSON.
- Use a estrutura: {"diagrams":[{"title":"...","type":"business_flow|user_journey|state_flow|decision_flow","description":"...","mermaid":"flowchart TD\\n..."}]}.
- Gere de 1 a 3 diagramas.
- Não gere diagramas genéricos sobre processo de requisitos, implementação, backlog ou a pipeline.
- Use termos concretos do domínio da reunião.
- Cada diagrama deve ser compreensível para pessoas de negócio e desenvolvimento.
- Cada nó deve ter texto curto, preferencialmente até 35 caracteres.
- Não crie diagramas com mais de 16 nós.
- Se o fluxo for grande, divida em múltiplos diagramas menores.
- Prefira flowchart TD para fluxos sequenciais simples.
- Prefira flowchart LR para jornadas com muitos atores.
- Evite caracteres que quebram Mermaid em labels, como aspas não escapadas, parênteses complexos e quebras de linha dentro dos nós.
- Use somente informações presentes nos requisitos. Se algo for suposição, deixe claro na description.

Requisitos refinados:
{{input}}`, now),

		newPrompt(domain.StageArtifactGeneration, `Você é um engenheiro de requisitos experiente.

Idioma da resposta: {{language}}.

Gere a documentação final de requisitos a partir dos requisitos refinados.

Use obrigatoriamente esta estrutura:

1. **Especificação de Requisitos de Software**
2. **Histórias de Usuário** (formato: Como [usuário], eu quero [funcionalidade] para [benefício])
3. **Critérios de Aceitação**
4. **Casos de Uso** (principais atores e fluxos principais)

Requisitos refinados:
{{input}}

Mantenha linguagem clara, objetiva e profissional.`, now),
	}
}

func Registry(defaults []domain.Prompt) map[domain.StageName]domain.Prompt {
	registry := make(map[domain.StageName]domain.Prompt, len(defaults))
	for _, prompt := range defaults {
		registry[prompt.StageName] = prompt
	}
	return registry
}

func Render(template string, values map[string]string) string {
	result := template
	for key, value := range values {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

func newPrompt(stage domain.StageName, template string, now time.Time) domain.Prompt {
	return domain.Prompt{
		ID:          domain.NewID(),
		StageName:   stage,
		Version:     VersionV1,
		Description: string(stage) + " default MVP prompt",
		Template:    template,
		CreatedAt:   now,
	}
}
