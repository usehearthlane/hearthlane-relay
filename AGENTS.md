# AGENTS.md

Guia de orientação para agentes de desenvolvimento (OpenCode e similares) que trabalharão neste repositório.

## Objetivo do projeto

Este projeto é um pequeno serviço privado de *relay* para localização de dispositivos Hearthlane. O relay permite que um dispositivo publique a própria localização mesmo com o aplicativo Hearthlane fechado, e que outro dispositivo consulte a última localização conhecida.

O projeto é **independente** do repositório do Hearthlane. O Hearthlane é apenas um cliente desse serviço.

## Estado atual

Servidor mínimo **implementado** (Fase 1), também com persistência (Fase 2), decisão de trust boundary: MVP sem autenticação própria (Fase 3) e testes (Fase 4). Arquivos de deploy Docker/Compose criados em `deploy/` (Fase 5). O relay é escrito em Go usando apenas a biblioteca padrão: `net/http`, `encoding/json`, `os`, `sync`, `time`. Pendente: deploy real no homelab e Fase 6 (integração Hearthlane).

(NOTA: o `Dockerfile` e o `compose.yml` presentes na raiz pertencem ao ambiente de desenvolvimento do OpenCode, não ao relay. Não devem ser tratados como infraestrutura do relay nem editados como parte deste projeto. A infraestrutura de deploy do relay vive em `deploy/`.)

## Limites de escopo

O relay é pequeno e deliberado. Seus limites são definidos no `README.md`. De forma resumida:

- O relay mantém somente a **última localização conhecida** de cada dispositivo.
- O relay mantém uma lista dos dispositivos conhecidos.
- O relay mantém um **apelido amigável** opcional por dispositivo.
- O relay fornece `publishedAtEpochMs` para que o cliente decida se a localização está available, stale ou unavailable.
- O relay atende 4 endpoints HTTP: publicação de localização, listagem de dispositivos, consulta de localização e atualização de nickname.

### Fora do escopo (NÃO implementar)

Nunca implementar, nem mesmo "por precaução":

- Histórico, trilha, rastreamento, eventos, auditoria de coordenadas ou backups internos de posições.
- Banco de dados (PostgreSQL, SQLite), Redis ou qualquer cache externo.
- Sistema de usuários, contas, membros da família, grupos, roles ou ACL complexa.
- OAuth, JWT, sessões ou qualquer mecanismo de autenticação no MVP (exceção documentada: se for necessária no futuro, usar credenciais individuais por dispositivo).
- UI, dashboards, mapas, geofencing, notificações, analytics ou telemetria.
- Descoberta automática de dispositivos, UPnP, abertura de portas ou exposição pública.
- Integração específica com Tailscale, Google Maps, Google Play Services ou qualquer serviço externo.
- Código, módulos ou classes importados do Hearthlane.
- Acoplamento ao Android (o relay não conhece Android).
- Localização direta dispositivo-a-dispositivo (P2P).
- Múltiplas instâncias, replicação, failover, load balancing, cluster ou Kubernetes.

## Princípios arquiteturais

1. **Última localização apenas.** Para cada dispositivo, uma publicação substitui completamente a anterior. Nunca armazenar listas de posições nem timestamps anteriores.
2. **`publishedAtEpochMs` é gerado pelo relay.** O cliente nunca controla esse campo.
3. **`deviceId` é a identidade estável.** Nickname é apenas metadado de apresentação, nunca identidade.
4. **O relay não decide sobre presença.** Available/Stale/Unavailable e os thresholds associados pertencem ao Hearthlane (cliente).
5. **Estado em memória é a fonte de verdade das consultas.** Persistência em arquivo JSON é somente sobrevivência ao restart.
6. **Concorrência segura.** Leituras concorrentes, escrita consistente, respostas completas, gravação atômica e ausência de corrupção JSON em atualizações simultâneas.
7. **Tailscale é responsabilidade da infraestrutura.** O relay é apenas um servidor HTTP privado e não conhece Tailscale.
8. **Simplicidade operacional.** Preferir a solução mais simples que funcione para 4–10 dispositivos.

## Regras de privacidade

Localização é dado sensível. Adotar minimização desde o início:

- **Coordenadas jamais em logs** (latitude/longitude, payload completo de localização, histórico).
- Nenhum histórico, nenhuma trilha.
- Nenhum endpoint público.
- Nenhuma integração cloud ou dependência de terceiros para armazenamento.
- Nenhuma exportação automática, analytics ou telemetria externa.
- Nenhum analytics de localização.
- Mensagens de erro **não** devem vazar coordenadas.

## Regras de armazenamento

- Estado JSON em memória.
- Persistência em arquivo JSON no disco.
- Gravação atômica (escrever em arquivo temporário e renomear, ou equivalente).
- Leitura do arquivo na inicialização.
- Persistência após cada alteração.
- O arquivo em disco representa **somente** o estado atual. Nunca conter lista de posições, histórico ou eventos.
- Se o arquivo de estado for inválido/corrompido na inicialização, o relay **não deve** apagar ou resetar os dados silenciosamente; deve falhar ao iniciar com erro explícito, exigindo intervenção da operação.
- **Proibido usar Redis, SQLite, PostgreSQL, MySQL ou qualquer banco/cache externo no MVP.** Somente uma necessidade real futura deve motivar essas tecnologias — nunca "por precaução".

## Contrato HTTP

O contrato é a fronteira com o Hearthlane. Deve permanecer estável. Não alterar sem justificativa e registro.

| Método | Rota | Descrição | Sucesso |
|---|---|---|---|
| PUT | `/devices/{deviceId}/location` | Publica/substitui a última localização do dispositivo | `204 No Content` |
| GET | `/devices` | Lista os dispositivos conhecidos | `200 OK` |
| GET | `/devices/{deviceId}/location` | Retorna a última localização conhecida | `200 OK`; `404` se não houver localização |
| PUT | `/devices/{deviceId}/nickname` | Atualiza o apelido do dispositivo | `204 No Content` |

### Regras do contrato

- `{deviceId}` é uma string estável fornecida pelo cliente. O relay não gera nem deriva esse ID internamente (exemplo Hearthlane: `hearthlane-<nodeSuffix>`).
- Corpo de publicação de localização: `latitude` e `longitude` obrigatórias; `accuracy` e `provider` opcionais; `recordedAtEpochMs` indica quando o dispositivo obteve a posição.
- O relay adiciona `publishedAtEpochMs` (momento em que recebeu/publicou a posição). O cliente não envia esse campo; se enviar, deve ser ignorado ou rejeitado.
- Publicar localização não apaga nickname; alterar nickname não altera localização.
- Remover/limpar nickname deve ser suportado pelo contrato.
- `GET /devices` não deve incluir coordenadas se não forem necessárias; deve incluir `deviceId`, `nickname`, indicação de existência de localização e `publishedAtEpochMs` (e eventualmente `accuracy`).
- O relay não implementa autenticação própria: os endpoints respondem sem `Authorization`, e a segurança é responsabilidade da rede privada (ver README seção 13).

## Regras de compatibilidade com Hearthlane

- O Hearthlane é cliente HTTP do relay. A única fronteira entre os projetos é o contrato HTTP.
- Não copiar código do Hearthlane para este projeto.
- Não criar dependência de módulos do Hearthlane.
- Não importar classes do Hearthlane.
- Não acoplar o projeto ao Android.
- Não conhecer a lógica de negócio do Hearthlane além do mínimo do contrato.
- Alterações no contrato devem manter compatibilidade com o cliente Hearthlane ou ser documentadas como quebra acordada com o cliente.

## Regras de trust boundary e autenticação

- O MVP **não implementa autenticação própria**. O relay aceita requisições sem `Authorization` e ignora qualquer cabeçalho de autenticação enviado.
- O serviço deve operar somente em rede privada confiável (LAN doméstica e/ou Tailscale). **Nunca** expor o relay diretamente à Internet.
- O Nginx Proxy Manager pode servir como reverse proxy da rede privada, mas não adiciona autenticação nem torna o serviço público.
- Mesmo sem validação, tokens e o cabeçalho `Authorization` jamais devem ser registrados em logs nem devolvidos em respostas.
- Evolução futura documentada (não implementar agora): credenciais individuais por dispositivo (`device A → credential A`), nunca um token único compartilhado.
- Não criar sistema de contas, OAuth ou JWT.

## Regras de logs

Pode-se registrar, no futuro:

- Startup e shutdown.
- Erros HTTP.
- Erros de persistência.
- Quantidade de dispositivos.
- Status operacional.

Nunca registrar:

- Latitude, longitude, histórico ou payload completo de localização.
- Tokens ou o cabeçalho `Authorization`.

## Regras de dependências

- Manter a árvore de dependências mínima.
- Nenhuma dependência de nuvem ou de serviços externos (Google Maps, Google Play Services, etc.).
- Nenhuma dependência de SDK do Tailscale.
- Nenhuma dependência de banco de dados ou Redis no MVP.
- Cada dependência adicionada deve ter justificativa registrada e alinhada ao escopo.

## Regras de simplicidade

- Se uma solução mais complexa não resolve um problema real já existente, não usar a solução complexa.
- Não antecipar complexidade (Redis, banco, cluster, usuários, históricos).
- Preferir: estado em memória + arquivo JSON + servidor HTTP simples.
- Volume esperado: aproximadamente 4–10 dispositivos, consultas triviais.

## Regras para alterações do contrato

- Não alterar o contrato HTTP sem justificar e documentar.
- Registrar a mudança no README.
- Manter compatibilidade com o cliente Hearthlane.
- Mudanças que removem ou renomeiam campos existentes devem ser tratadas como quebra de contrato.

## Regras para testes futuros

Quando a implementação iniciar (Fase 4):

- Testes unitários das regras de modelo (substituição completa de localização, geração de `publishedAtEpochMs`).
- Testes HTTP de todos os endpoints do contrato (incluindo `404` e validação).
- Testes de persistência (gravação, leitura na inicialização, gravação atômica).
- Testes de concorrência (atualizações simultâneas não corrompem o estado nem o JSON).
- Testes de recuperação após restart.
- Testes devem verificar que coordenadas e tokens não chegam a logs.

## Proibições explícitas

- Proibido introduzir histórico, trilha, eventos ou auditoria de coordenadas.
- Proibido registrar coordenadas em logs.
- Proibido depender de cloud ou criar endpoint público.
- Proibido acoplar ao Hearthlane (código, módulos, classes, Android).
- Proibido criar integração específica com Android.
- Proibido usar Redis/banco de dados antes de uma necessidade real e registrada.
- Proibido criar usuários, roles, ACL complexa, OAuth ou JWT.
- Proibido implementar P2P, descoberta de dispositivos, mapas, geofencing ou notificações.

## Antes de implementar

Antes de escrever qualquer código neste projeto, o agente deve:

1. Ler este `AGENTS.md`.
2. Ler o `README.md`.
3. Entender o contrato HTTP e o modelo de dados.
4. Verificar se alguma decisão arquitetural já foi registrada (no `README.md`, seção de decisões arquiteturais) e respeitá-la.
5. Não implementar funcionalidades fora do escopo documentado.
6. Não alterar o contrato HTTP sem justificar e documentar.
7. Manter compatibilidade com o cliente Hearthlane.
8. Confirmar que a solução escolhida permanece pequena, simples e fácil de operar.

Quando houver dúvida entre implementar algo que "pode ser útil" e manter o escopo mínimo, o escopo mínimo vence.