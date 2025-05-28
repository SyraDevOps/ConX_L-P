# Como Compilar Arquivos Go

Este guia ensina como compilar arquivos Go no terminal do Linux, macOS e Windows, além de gerar executáveis para diferentes sistemas operacionais (cross-compilation).

---

## Compilando no Terminal

### Linux/macOS

Abra o terminal e execute:

```sh
go build -o programa main.go
```

- `programa` será o executável gerado.
- `main.go` é o arquivo principal do seu projeto.

### Windows (Prompt de Comando)

Abra o Prompt de Comando (cmd) e execute:

```cmd
go build -o programa.exe main.go
```

---

## Compilando para Outros Sistemas (Cross-compilation)

Você pode compilar para outros sistemas operacionais e arquiteturas usando as variáveis de ambiente `GOOS` e `GOARCH`.

### Exemplos

#### Compilar para Windows no Linux/macOS

```sh
GOOS=windows GOARCH=amd64 go build -o programa.exe main.go
```

#### Compilar para Linux no Windows (cmd)

```cmd
set GOOS=linux
set GOARCH=amd64
go build -o programa main.go
```

#### Compilar para macOS no Linux

```sh
GOOS=darwin GOARCH=amd64 go build -o programa main.go
```

---

## Tabela de Valores Comuns

| Sistema Operacional | GOOS     | GOARCH  | Extensão do Executável |
|---------------------|----------|---------|-----------------------|
| Windows             | windows  | amd64   | .exe                  |
| Linux               | linux    | amd64   | (sem extensão)        |
| macOS               | darwin   | amd64   | (sem extensão)        |

---

## Observações

- Certifique-se de ter o Go instalado e configurado no PATH.
- Para compilar para outras arquiteturas (ex: ARM), altere o valor de `GOARCH` (ex: `arm64`).
- No PowerShell, use `$env:GOOS="windows"` e `$env:GOARCH="amd64"` antes do comando `go build`.
