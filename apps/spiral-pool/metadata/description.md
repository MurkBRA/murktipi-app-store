# Spiral Mining Pool

**Spiral Pool** é um servidor de pool de mineração solo e multi-moedas de alta performance, projetado para mineradores que desejam minerar diretamente em seus próprios nós de blockchain com soberania total e taxa zero de pool.

---

## Principais Funcionalidades

- **Mineração Solo e Multi-Moedas:** Suporte a mineração dedicada de uma moeda (`single`) ou múltiplas moedas simultaneamente no mesmo servidor (`multi`).
- **Algoritmos Suportados:**
  - **SHA-256d:** Bitcoin (BTC), DigiByte (DGB), Bitcoin Cash (BCH), Namecoin (NMC), Myriadcoin (XMY), Fractal Bitcoin (FBTC), eCash (XEC).
  - **Scrypt:** Litecoin (LTC), Dogecoin (DOGE), DigiByte-Scrypt, PepeCoin (PEP).
- **Merge Mining:** Minere LTC + DOGE ou BTC + NMC sem perda de taxa de hash.
- **Protocolos Stratum:**
  - Stratum V1 tradicional em texto claro (`stratum+tcp://`).
  - Stratum V2 Noise Protocol com criptografia ponta a ponta para proteger contra spoofing e interceptações de shares.
- **Dashboard Web & Métricas:** Interface administrativa em tempo real com estatísticas de hashrate, shares aceitos/rejeitados, blocos minerados e latência dos trabalhadores.
- **Exportador Prometheus Integrado:** Pronto para monitoramento com Grafana.

---

## Configuração e Conexão de Mineradores

Ao instalar pelo instalador do Runtipi, selecione o modo de operação desejado:

### 1. Conexão Stratum V1
Aponte seu minerador ASIC (Antminer, Whatsminer, Avalon) ou software (CGMiner, BFGMiner) para:
- **URL / Host:** `IP_DO_SEU_SERVIDOR_RUNTIPI`
- **Porta:** A porta configurada no instalador (Padrão BTC: `4333`, DGB: `3333`, LTC: `7333`, DOGE: `8335`)
- **Usuário (Worker):** `SEU_ENDEREÇO_DE_CARTEIRA.nome_do_minerador`
- **Senha:** `x` (ou qualquer senha arbitrária)

Exemplo:
```text
URL: stratum+tcp://192.168.1.100:4333
User: bc1qexemploenderecodecarteiraaqui.asic1
Password: x
```

### 2. Acesso ao Dashboard
O Dashboard administrativo está acessível na porta configurada pelo Runtipi (`1618`).
- **Autenticação:** Ativada por padrão com a senha definida no campo `DASHBOARD_ADMIN_PASSWORD`.
