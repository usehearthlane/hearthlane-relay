# Hearthlane Relay

Relay privado de localização de dispositivos para o Hearthlane.

## 1. Nome do projeto

**Hearthlane Relay** (referência de repositório: `hearthlane-relay`).

Um serviço pequeno e privado de *relay* de localização, independente do projeto Hearthlane.

## 2. Descrição

O Hearthlane Relay é um serviço HTTP mínimo que recebe a localização publicada por dispositivos Hearthlane, mantém somente a **última localização conhecida** de cada dispositivo e disponibiliza essa informação para outros dispositivos consultarem posteriormente.

Ele não é um sistema de usuários, não mantém histórico e não decide sobre a "saúde" de uma localização. Ele apenas responde a uma pergunta:

> "Qual foi a última localização publicada por cada dispositivo, e quando o relay a recebeu?"

## 3. Motivação

Um dispositivo Android com Hearthlane deve poder publicar a própria localização **mesmo quando o aplicativo Hearthlane não está aberto**. Outro dispositivo Hearthlane deve poder consultar essa localização posteriormente, sem depender de uma conexão direta com o primeiro dispositivo.

Isso exige um ponto intermediário mínimo: um relay privado, executado na infraestrutura do usuário (homelab), sem cloud pública, sem endpoint público e sem dependência de serviços externos.

## 4. Objetivo

- Receber a localização atual de um dispositivo.
- Armazenar somente a última localização conhecida de cada dispositivo.
- Disponibilizar a lista de dispositivos conhecidos.
- Disponibilizar a última localização conhecida de um dispositivo.
- Permitir um apelido amigável (nickname) opcional por dispositivo.
- Fornecer `publishedAtEpochMs` para que o cliente determine se a localização está available, stale ou unavailable.

O relay é deliberadamente pequeno, simples e fácil de operar. Universo esperado: **aproximadamente 4 a 10 dispositivos**.

## 5. Arquitetura conceitual

```
            ┌─────────────────────────────┐
            │         HOMELAB             │
            │                             │
            │   ┌───────────────────┐     │
            │   │  Relay HTTP       │     │
            │   │  (estado em memória)     │
            │   │  └ JSON em disco  │     │
            │   └───────────────────┘     │
            └─────────────────────────────┘
                ▲                   ▲
                │                   │
      publica   │                   │   consulta
                │                   │
   Device A (Hearthlane)   Device B (Hearthlane)
```

- Estado em memória é a fonte de verdade das consultas.
- Arquivo JSON em disco serve apenas como persistência (sobrevivência ao restart).
- O relay é acessível via LAN e via Tailscale, mas **não conhece Tailscale**.

## 6. Fluxo Device → Relay → Device

1. O **Device A** coleta a localização (com o Hearthlane aberto ou em segundo plano) e publica no relay.
2. O relay valida, grava no estado em memória e persiste.
3. O **Device B** consulta o relay e recebe a última localização conhecida do Device A.

```
Device A ──PUT /devices/{id}/location──▶ Relay ──persistir──▶ JSON em disco
                                               ▲
Device B ──GET  /devices/{id}/location───────┘
```

Este modelo é preferido ao direto:

```
Device A ──▶ Device B   (NÃO é o modelo deste projeto)
```

Com o relay como intermediário:

- o Device A pode estar com o Hearthlane fechado (background location no Android);
- o Device B consulta mesmo sem o A estar online no momento;
- não é necessária conexão P2P;
- o Device A não atua como servidor;
- o node Tailscale do dispositivo A não precisa ficar disponível para consultas.

## 7. O que o relay faz

- Mantém a última localização conhecida por dispositivo.
- Gera e armazena `publishedAtEpochMs` no momento da publicação.
- Mantém a lista de dispositivos conhecidos.
- Mantém um nickname opcional por dispositivo.
- Suporta limpar o nickname de um dispositivo.
- Responde consultas de localização e listagem diretamente do estado em memória.
- Persiste o estado atual em arquivo JSON após cada alteração.

## 8. O que o relay NÃO faz

- Não armazena histórico, trilhas, rastreamento, eventos ou auditoria de coordenadas.
- Não registra coordenadas em logs.
- Não calcula rotas, não gera mapas, não faz geofencing nem notificações.
- Não determina presença nem decide se uma localização está "correta".
- Não define thresholds de available/stale/unavailable — isso pertence ao Hearthlane.
- Não depende de Google Maps, Google Play Services ou qualquer serviço externo.
- Não faz polling dos dispositivos nem tenta localizá-los por conta própria.
- Não conhece Tailscale, UPnP, nem expõe porta pública.
- Não possui sistema de usuários, contas, grupos, roles ou ACL complexa.
- Não usa OAuth, JWT ou sessões.
- Não implementa localização P2P dispositivo-a-dispositivo.
- Não conhece a lógica de negócio do Hearthlane além do mínimo do contrato.

## 9. Modelo de dados

O conceito central é um **Device**, identificado por um `deviceId` estável. Para o relay, `deviceId` é apenas uma string — o relay não gera nem deriva esse ID (exemplo Hearthlane: `hearthlane-<nodeSuffix>`).

Estado conceitual persistido por dispositivo:

```
Device:
- deviceId
- nickname
- location

Location:
- latitude              (obrigatória quando há localização)
- longitude             (obrigatória quando há localização)
- accuracy              (opcional / nula)
- provider              (opcional / nulo)
- recordedAtEpochMs     (quando o dispositivo obteve a posição)
- publishedAtEpochMs    (quando o relay recebeu/publicou — gerado pelo relay)
```

Regras do modelo:

- `publishedAtEpochMs` é gerado exclusivamente pelo relay. O cliente não controla esse campo.
- Uma publicação **substitui completamente** a anterior.
- Nunca existe lista de posições nem histórico implícito.

Exemplo conceitual (apenas ilustrativo — não é implementação):

```json
{
  "devices": {
    "hearthlane-ab12cd34": {
      "nickname": "Meu celular",
      "location": {
        "latitude": -23.55,
        "longitude": -46.63,
        "accuracy": 50,
        "provider": "network",
        "recordedAtEpochMs": 1234567890000,
        "publishedAtEpochMs": 1234567895000
      }
    }
  }
}
```

### Nickname

- Apelido opcional, apenas metadado de apresentação. Exemplos: "Meu celular", "Celular da mãe", "Tablet", "iPhone".
- `deviceId` continua sendo a identidade; nickname nunca é identificador.
- Alterar nickname não altera localização; publicar localização não apaga nickname.
- Remover/limpar o nickname é suportado pelo contrato.

## 10. Contrato HTTP

O contrato é a fronteira entre o relay e o Hearthlane. Implementado na Fase 1 e mantido estável.

| Método | Rota | Descrição | Resposta de sucesso |
|---|---|---|---|
| `PUT` | `/devices/{deviceId}/location` | Publica/substitui a última localização do dispositivo | `204 No Content` |
| `GET` | `/devices` | Lista os dispositivos conhecidos | `200 OK` |
| `GET` | `/devices/{deviceId}/location` | Retorna a última localização conhecida | `200 OK`; `404` se não houver localização |
| `PUT` | `/devices/{deviceId}/nickname` | Atualiza o apelido do dispositivo | `204 No Content` |

Regras:

- Corpo de publicação: `latitude` e `longitude` obrigatórias; `accuracy` e `provider` opcionais; `recordedAtEpochMs` opcional (quando o dispositivo obteve a posição).
- O relay rejeita/ignora `publishedAtEpochMs` enviado pelo cliente.
- `GET /devices` retorna informações mínimas de apresentação: `deviceId`, `nickname`, existência de localização, `publishedAtEpochMs` e eventualmente `accuracy`. **Não** expõe coordenadas na listagem.
- `GET /devices/{deviceId}/location` retorna a localização quando existir; `404` quando não houver.
- O relay não implementa autenticação própria: os endpoints respondem sem `Authorization`, e a segurança é garantida pela rede privada (ver seção 13).

## 11. Política de armazenamento — "last known only"

Para cada dispositivo, uma publicação substitui completamente a anterior:

- ANTES: `location A`
- DEPOIS: `location B`
- O relay guarda somente `B`.

Não são armazenados: `location A`, timestamps anteriores, arrays de posições, eventos, trilhas, auditoria de coordenadas ou backups internos de posições.

O arquivo JSON em disco representa **somente o estado atual**.

## 12. Política de privacidade

Localização é dado sensível. O projeto adota minimização desde o início:

- Nenhuma coordenada em logs.
- Nenhum histórico, nenhuma trilha, nenhum analytics de localização.
- Nenhum endpoint público.
- Nenhuma integração cloud.
- Nenhuma exportação automática.
- Nenhuma dependência de terceiros para armazenamento.
- Persistir apenas a última posição.
- Mensagens de erro não vazam coordenadas.

O relay foi projetado para operar **dentro da infraestrutura privada do usuário**.

## 13. Segurança e trust boundary

O Hearthlane Relay MVP **não implementa autenticação própria**. O serviço deve permanecer acessível somente em uma rede privada confiável, como a LAN doméstica e/ou Tailscale. O relay **não deve ser exposto diretamente à Internet**.

- O relay aceita requisições sem `Authorization`; qualquer cabeçalho de autenticação enviado é ignorado (não é validado nem rejeitado).
- A segurança é responsabilidade da infraestrutura: LAN/Tailscale, firewall, VPN. O relay não decide nem controla a exposição da rede.
- O Nginx Proxy Manager pode ser utilizado como reverse proxy na rede privada, mas isso **não** transforma o relay em serviço público — o relay não executa regras de autenticação e não deve ser publicado para a Internet.
- Mesmo sem validação, tokens e o cabeçalho `Authorization` jamais devem aparecer em logs nem ser devolvidos em respostas.

**Evolução futura (não implementada no MVP):** se autenticação for necessária, preferir credenciais individuais por dispositivo (`device A → credential A`, `device B → credential B`) a um token único compartilhado para todo o relay. Não fazem parte do desenho: contas, OAuth, JWT ou sessões.

## 14. Relação com o Hearthlane

- O Hearthlane é **cliente HTTP** do relay. A única fronteira entre os projetos é o **contrato HTTP**.
- Este projeto é independente: não copia código do Hearthlane, não depende de módulos, não importa classes e não se acopla ao Android.
- `deviceId` é fornecido pelo cliente (exemplo Hearthlane: `hearthlane-<nodeSuffix>`); o relay não conhece a geração interna desse ID.
- **Background location** é responsabilidade do aplicativo Android. Ao relay cabem apenas as publicações.
- O relay existe para que um dispositivo seja localizado sem que outro precise de conexão direta com ele.

## 15. Papel do Tailscale

- Clientes acessam o relay via LAN (na rede local) ou via Tailscale (remotamente).
- **O relay não conhece Tailscale.** Ele é apenas um servidor HTTP privado.
- A conectividade privada é responsabilidade da infraestrutura (rede, nó Tailscale, DNS, etc.).
- Não há SDK do Tailscale, integração específica, descoberta automática, UPnP, abertura de portas ou exposição pública.

## 16. Persistência

Implementado na Fase 1:

- Estado JSON em memória (fonte de verdade das consultas).
- Persistência em arquivo JSON no disco (somente estado atual).
- Gravação atômica (arquivo temporário + rename, ou equivalente).
- Leitura do arquivo na inicialização.
- Persistência após cada alteração.
- Se o arquivo não existir na primeira inicialização, inicia com estado vazio.
- Se o arquivo estiver inválido ou corrompido, o relay falha ao iniciar com erro explícito — nunca apaga os dados silenciosamente.

**Sem** banco de dados, Redis, SQLite, PostgreSQL ou cache externo no MVP. Redis só poderá ser considerado no futuro diante de uma necessidade real registrada — nunca "por precaução".

### Disponibilidade

- Não existe requisito de alta disponibilidade: sem cluster, replicação, failover, load balancing, Kubernetes ou múltiplas instâncias.
- Se o relay estiver indisponível: o Hearthlane continua funcionando; o dispositivo pode tentar publicar novamente; outros dispositivos não obtêm nova localização até o relay voltar; a última localização persistida volta após restart.

## 17. Escala esperada (4–10 dispositivos)

- Universo de aproximadamente 4 a 10 dispositivos.
- Consultas triviais, volume extremamente pequeno.
- Consulta direta sobre o estado em memória (rápida).
- Estrutura simples, backup e inspeção fáceis, menor superfície operacional.
- Sem otimização para grandes volumes.

## 18. Configuração

Configuração mínima por environment variables, suficiente para Docker:

| Variável | Padrão | Descrição |
|---|---|---|
| `RELAY_BIND` | `0.0.0.0` | Endereço de bind (a exposição da rede é responsabilidade da operação) |
| `RELAY_PORT` | `8080` | Porta HTTP |
| `RELAY_DATA_FILE` | `state.json` | Caminho do arquivo JSON de persistência |

O relay **não** possui variável de autenticação: não há token, chave ou segredo. A segurança é a rede privada (ver seção 13).

### Deploy no homelab (Docker / Fase 5)

Arquivos de deploy em `deploy/` (a raiz do repositório não contém infraestrutura do relay — os `Dockerfile`/`compose.yml` da raiz pertencem ao ambiente de desenvolvimento do OpenCode).

```bash
cd deploy
docker compose up -d --build
```

- Imagem multi-stage (build Go → runtime Alpine), roda como usuário **não-root** (UID 10001).
- O estado persiste no volume nomeado `relay-data`, montado em `/data/state.json` (sobrevive a restart/recreate; a última localização volta após reinício).
- `restart: unless-stopped`; porta publicada `8080:8080`. Sem autenticação própria, publique apenas em rede privada (LAN/Tailscale, firewall) — **nunca** exponha o contêiner diretamente à Internet (ver seção 13).

## 19. Decisões arquiteturais

Decisões registradas (não reverter sem justificativa e registro):

1. Relay é um projeto separado do Hearthlane.
2. Fase 0 entregou apenas documentação; o relay foi implementado na Fase 1 (servidor mínimo, persistência atômica e testes).
3. Relay mantém somente a última localização conhecida.
4. Não existe histórico.
5. Não existe trilha.
6. JSON em memória é suficiente.
7. Arquivo JSON é persistência.
8. Não usar Redis no MVP.
9. Não usar banco de dados no MVP.
10. Escala esperada de aproximadamente 4–10 dispositivos.
11. Consulta deve ser rápida, diretamente da memória.
12. Relay não conhece Tailscale.
13. Tailscale é responsabilidade da infraestrutura.
14. Hearthlane é cliente HTTP do relay.
15. Background location é responsabilidade do Hearthlane.
16. Relay serve como mediador/relay mínimo.
17. P2P não é o modelo principal.
18. Lista de devices é responsabilidade do relay.
19. Nickname é responsabilidade do relay.
20. `deviceId` é a identidade estável.
21. `publishedAtEpochMs` é gerado pelo relay.
22. Available/Stale/Unavailable são decisões do cliente.
23. Coordenadas não aparecem em logs.
24. Tokens não aparecem em logs.
25. Não existe cloud.
26. Não existe endpoint público.
27. Não existe sistema de usuários no MVP.
28. MVP sem autenticação própria: o relay confia na rede privada (LAN/Tailscale) como fronteira de confiança e não valida `Authorization`. Se autenticação for necessária no futuro, usar credenciais individuais por dispositivo — nunca um token único compartilhado.
29. Arquivo de estado corrompido/inválido na inicialização faz o relay falhar ao iniciar — nunca apaga os dados silenciosamente.
30. Configuração mínima por environment variables (`RELAY_BIND`, `RELAY_PORT`, `RELAY_DATA_FILE`).
31. Endpoint de health não foi adicionado: o contrato documentado define exatamente 4 endpoints.
32. Arquivos de deploy (Fase 5) ficam em `deploy/` para não conflitar com o `Dockerfile`/`compose.yml` do ambiente de desenvolvimento na raiz; o relay opera como binário ou contêiner não-root, com volume persistente.

## 20. Roadmap inicial

Fases 0 (documentação), 1 (servidor mínimo), 2 (persistência), 3 (segurança), 4 (testes) concluídas e Fase 5 (deploy) com os arquivos Docker/Compose criados. Pendente: deploy real no homelab e Fase 6 (integração Hearthlane).

### Fase 0 — Documentação e contrato (concluída)
- `AGENTS.md`
- `README.md`
- Contrato HTTP
- Modelo de dados

### Fase 1 — Servidor mínimo (concluída)
- Servidor HTTP
- `GET /devices`
- `GET /devices/{id}/location`
- `PUT /devices/{id}/location`
- `PUT /devices/{id}/nickname`

### Fase 2 — Persistência (concluída)
- JSON em memória
- Persistência em arquivo
- Gravação atômica
- Recuperação após restart

### Fase 3 — Segurança (concluída)
- Decisão de trust boundary: MVP **sem autenticação própria**, confiando na rede privada (LAN/Tailscale) como única fronteira
- Remoção do `Authorization: Bearer` e da variável `RELAY_TOKEN`
- Manutenção das regras de privacidade (sem coordenadas, tokens ou cabeçalho `Authorization` em logs)

### Fase 4 — Testes (concluída)
- Testes unitários
- Testes HTTP
- Testes de persistência
- Concorrência
- Recuperação após restart

### Fase 5 — Deploy (arquivos concluídos; pendente deploy no homelab)
- Docker (`deploy/Dockerfile`)
- Docker Compose (`deploy/compose.yml`)
- Volume persistente (`relay-data`)
- Configuração (environment variables)
- Documentação de deploy no homelab

### Fase 6 — Integração Hearthlane (pendente)
- Integração do cliente Android
- Publicação em background
- Consulta da lista de devices
- Consulta de localização
- Nickname
- Estados Available/Stale/Unavailable

## 21. Estado atual do projeto

**Este projeto está na Fase 1 concluída**, com persistência (Fase 2), decisão de trust boundary sem autenticação própria (Fase 3), testes (Fase 4) e os arquivos de deploy (Fase 5) também entregues.

- O servidor HTTP está implementado em Go (biblioteca padrão), com os 4 endpoints do contrato.
- Estado em memória é a fonte de verdade das consultas; persistência atômica em arquivo JSON.
- Sem autenticação própria: os endpoints respondem sem `Authorization` e o relay deve operar somente na rede privada (ver seção 13).
- Testes abrangentes (unitários, HTTP, persistência, concorrência e privacidade dos logs) passam com `go test -race`.
- Deploy via Docker/Compose em `deploy/` (ver seção 18); pendente o deploy real no homelab e a integração com o cliente Hearthlane (Fase 6).
- O repositório contém somente o relay. O `Dockerfile` e o `compose.yml` na raiz pertencem ao ambiente de desenvolvimento do OpenCode, não ao relay.

O objetivo desta fase foi deixar um servidor mínimo, com o contrato e os limites arquiteturais respeitados, pronto para as próximas fases de deploy e integração.