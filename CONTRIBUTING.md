# Contribuindo

Contribuições são bem-vindas por meio de issues e pull requests.

## Antes de começar

- Use Go 1.26 ou mais recente.
- Não inclua chaves de API, arquivos `.env`, áudios, transcrições, dados pessoais ou artefatos gerados.
- Abra uma issue antes de mudanças grandes para alinhar escopo e abordagem.
- Use dados sintéticos nos testes e exemplos.

## Desenvolvimento

1. Faça um fork do repositório e crie uma branch a partir de `main`.
2. Implemente a menor alteração que resolva o problema.
3. Formate arquivos Go alterados com `gofmt`.
4. Execute as verificações locais:

```bash
go vet ./...
go test ./...
```

5. Abra um pull request descrevendo o problema, a solução e como ela foi validada.

Testes automatizados não precisam de OpenAI nem MongoDB: eles usam providers mockados e stores em memória.

## Relatos de segurança

Não divulgue vulnerabilidades em issues públicas. Siga as instruções de [SECURITY.md](SECURITY.md).

Ao enviar uma contribuição, você concorda que ela será disponibilizada sob a [Licença MIT](LICENSE) do projeto.
