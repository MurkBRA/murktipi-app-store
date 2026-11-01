# Technitium DNS Server

**Technitium DNS Server** é um servidor DNS autoritativo e recursivo de código aberto e nível corporativo, ideal para privacidade, segurança de rede local, resolução de domínios privados e bloqueio de anúncios e rastreadores em toda a sua rede (estilo Pi-hole / AdGuard Home, porém com recursos avançados de DNS).

---

## Principais Recursos

- **Resolução Autoritativa e Recursiva:** Crie suas próprias zonas DNS primárias, secundárias e reversas com suporte completo a registros (A, AAAA, CNAME, PTR, TXT, MX, SRV, CAA, SSHFP, etc.).
- **Bloqueio de Anúncios e Telemetria:** Baixa e atualiza listas de bloqueio populares automaticamente, protegendo todos os dispositivos da sua casa ou empresa sem necessidade de instalar extensões em cada cliente.
- **Protocolos Modernos Criptografados:**
  - **DoH (DNS-over-HTTPS)** e HTTP/3
  - **DoT (DNS-over-TLS)**
  - **DoQ (DNS-over-QUIC)**
- **Console Web Completo:** Interface gráfica intuitiva com logs de consultas em tempo real, gráficos de estatísticas, gerenciamento de registros e configurações de encaminhadores (forwarders).
- **Autenticação Avançada:** Suporte integrado a Single Sign-On (SSO via OpenID Connect) e integração com Active Directory / OpenLDAP.
- **Suporte a Servidor DHCP:** Pronto para atuar como servidor DHCP local caso desejado.

---

## Acesso Inicial e Credenciais

- **URL de Acesso:** `http://<IP-DO-SEU-RUNTIPI>:<APP_PORT>` (Porta padrão Runtipi ou `5380`)
- **Usuário Padrão:** `admin`
- **Senha Inicial:** A senha configurada no instalador (`DNS_SERVER_ADMIN_PASSWORD`).
  > **Nota Importante:** As variáveis de ambiente do Technitium são lidas **apenas na primeira inicialização** para gerar a configuração inicial. Alterações subsequentes podem ser feitas diretamente pelo painel Web ou pela API HTTP.

---

## Configurando seus dispositivos

Para que seus dispositivos (roteador, computadores, celulares) utilizem o Technitium DNS:
1. Acesse as configurações de rede do seu roteador (ou do dispositivo individual).
2. Altere o servidor DNS primário (IPv4) para o **IP do seu servidor Runtipi**.
3. Como DNS secundário, você pode deixar vazio ou apontar para outro nó DNS de sua preferência.
