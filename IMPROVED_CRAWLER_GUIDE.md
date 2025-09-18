# Sistema de Crawler Melhorado com Padrões de Referência

## Visão Geral

O sistema de crawler melhorado utiliza o arquivo `List-site.ini` como base de treinamento para identificar com maior precisão páginas de anúncios de imóveis. Este documento explica como usar e configurar o novo sistema.

## Principais Melhorias

### 1. **Sistema de Treinamento Baseado em Referência**
- Usa URLs conhecidas do `List-site.ini` para aprender padrões específicos de cada site
- Identifica seletores CSS que funcionam consistentemente
- Cria padrões de URL para reconhecimento automático

### 2. **Classificação Multi-Camadas**
- **Classificador Rigoroso**: Critérios mais específicos para evitar falsos positivos
- **Classificador de Conteúdo**: Analisa características do texto da página
- **Validador de Padrões**: Usa padrões aprendidos para validação

### 3. **Extrator Melhorado**
- Seletores CSS hierárquicos (primários, secundários, fallback)
- Seletores específicos por domínio baseados no treinamento
- Validação de conteúdo extraído

### 4. **Sistema de Validação**
- Valida se URLs realmente contêm anúncios de imóveis
- Calcula confiança baseada em múltiplos fatores
- Cache de resultados para otimização

## Como Usar

### 1. **Preparação do Ambiente**

```bash
# Compile o crawler melhorado
go build -o bin/improved_crawler cmd/improved_crawler/main.go
```

### 2. **Treinamento Inicial**

```bash
# Treina padrões usando List-site.ini (apenas treinamento)
./bin/improved_crawler -reference=List-site.ini -train-only

# Treina e executa crawling
./bin/improved_crawler -reference=List-site.ini
```

### 3. **Opções de Linha de Comando**

```bash
# Opções disponíveis
./bin/improved_crawler -h

Flags:
  -config string
        Path to config file
  -reference string
        Path to reference URLs file (default "List-site.ini")
  -train-only
        Only train patterns, don't crawl
  -stats
        Show crawler statistics during execution
```

### 4. **Exemplo de Uso Completo**

```bash
# Executa com estatísticas em tempo real
./bin/improved_crawler -reference=List-site.ini -stats

# Usa arquivo de configuração específico
./bin/improved_crawler -config=config.json -reference=List-site.ini
```

## Arquitetura do Sistema

### Componentes Principais

1. **ReferencePatternTrainer**
   - Analisa URLs do `List-site.ini`
   - Extrai padrões de URL e seletores CSS
   - Consolida padrões por domínio

2. **PatternValidator**
   - Valida se URLs são realmente páginas de anúncios
   - Usa padrões aprendidos para classificação
   - Mantém cache de resultados

3. **EnhancedExtractor**
   - Extrai dados usando seletores hierárquicos
   - Aplica seletores específicos por domínio
   - Valida qualidade dos dados extraídos

4. **ImprovedCrawler**
   - Orquestra todos os componentes
   - Mantém estatísticas detalhadas
   - Suporte a cancelamento gracioso

### Fluxo de Processamento

```
1. Treinamento
   ├── Carrega URLs do List-site.ini
   ├── Visita cada URL e analisa estrutura
   ├── Extrai seletores CSS eficazes
   └── Consolida padrões por domínio

2. Crawling
   ├── Carrega URLs iniciais do sites.json
   ├── Para cada página encontrada:
   │   ├── Classifica tipo (propriedade/catálogo)
   │   ├── Usa padrões aprendidos se disponível
   │   ├── Extrai dados com seletores apropriados
   │   └── Valida qualidade dos dados
   └── Salva propriedades válidas
```

## Configuração do List-site.ini

O arquivo `List-site.ini` deve conter URLs de páginas de anúncios individuais, uma por linha:

```ini
https://site1.com.br/imovel/123456
https://site2.com.br/propriedade/789012
https://site3.com.br/casa/345678
# Comentários são ignorados
https://site4.com.br/apartamento/901234
```

### Boas Práticas para o List-site.ini

1. **Diversidade de Sites**: Inclua URLs de diferentes sites imobiliários
2. **Tipos Variados**: Misture casas, apartamentos, terrenos, etc.
3. **URLs Específicas**: Use URLs de anúncios individuais, não de listagens
4. **Atualização Regular**: Mantenha URLs válidas e atualizadas

## Monitoramento e Estatísticas

### Estatísticas Disponíveis

- **Páginas Visitadas**: Total de páginas processadas
- **Propriedades Encontradas**: Páginas identificadas como anúncios
- **Propriedades Salvas**: Anúncios com dados suficientes salvos
- **Páginas de Catálogo**: Páginas de listagem identificadas
- **Erros**: Problemas encontrados durante o processo
- **Estatísticas por Domínio**: Breakdown por site processado

### Exemplo de Saída

```
INFO[2024-01-15T10:30:00Z] Final crawling statistics
  avg_pages_per_min=12.5
  catalog_pages=45
  domains_processed=8
  errors=3
  pages_visited=150
  properties_found=89
  properties_saved=85
  success_rate=95.51%
  total_duration=12m30s
```

## Solução de Problemas

### Problemas Comuns

1. **Baixa Taxa de Sucesso**
   - Verifique se URLs no `List-site.ini` estão válidas
   - Execute apenas treinamento primeiro: `-train-only`
   - Analise logs para identificar padrões de erro

2. **Muitos Falsos Positivos**
   - O sistema usa classificação rigorosa por padrão
   - Ajuste thresholds no código se necessário
   - Verifique qualidade das URLs de referência

3. **Dados Extraídos Incompletos**
   - Adicione mais URLs de referência do mesmo domínio
   - Verifique se seletores CSS estão sendo aprendidos corretamente
   - Use logs de debug para análise detalhada

### Logs de Debug

Para habilitar logs detalhados, configure o nível de log:

```bash
export LOG_LEVEL=debug
./bin/improved_crawler -reference=List-site.ini
```

## Integração com Sistema Existente

O crawler melhorado é compatível com o sistema existente:

- Usa o mesmo repositório MongoDB
- Mantém estrutura de dados `Property` inalterada
- Pode ser usado em paralelo com o crawler original
- Suporte ao serviço de IA (Gemini) existente

## Próximos Passos

1. **Teste com URLs Conhecidas**: Execute primeiro com `-train-only` para validar padrões
2. **Monitoramento Inicial**: Use `-stats` para acompanhar performance
3. **Ajuste Fino**: Adicione mais URLs de referência conforme necessário
4. **Produção**: Integre ao pipeline de crawling existente

## Contribuição

Para melhorar o sistema:

1. Adicione URLs de qualidade ao `List-site.ini`
2. Reporte problemas com sites específicos
3. Sugira melhorias nos algoritmos de classificação
4. Contribua com novos seletores CSS para sites não cobertos
