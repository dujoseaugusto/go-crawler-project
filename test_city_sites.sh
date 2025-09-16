#!/bin/bash

# Script de Teste - Sistema de Sites por Cidade
# Uso: ./test_city_sites.sh

API_URL="http://localhost:8080"
CONTENT_TYPE="Content-Type: application/json"

echo "🚀 Iniciando testes do Sistema de Sites por Cidade"
echo "================================================="

# Função para fazer requisições e mostrar resposta
make_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4
    
    echo ""
    echo "📋 Teste: $description"
    echo "🔗 $method $endpoint"
    
    if [ -n "$data" ]; then
        echo "📤 Dados: $data"
        response=$(curl -s -X $method "$API_URL$endpoint" -H "$CONTENT_TYPE" -d "$data")
    else
        response=$(curl -s -X $method "$API_URL$endpoint")
    fi
    
    echo "📥 Resposta:"
    echo "$response" | jq '.' 2>/dev/null || echo "$response"
    echo "----------------------------------------"
}

# Aguarda o servidor iniciar
echo "⏳ Aguardando servidor iniciar..."
sleep 3

# 1. Verificar se o servidor está funcionando
echo ""
echo "1️⃣ VERIFICAÇÃO INICIAL"
make_request "GET" "/health" "" "Health Check - Verificar se servidor está rodando"

# 2. Listar cidades existentes (deve estar vazio inicialmente)
echo ""
echo "2️⃣ ESTADO INICIAL"
make_request "GET" "/cities" "" "Listar cidades existentes"

# 3. Descobrir sites para algumas cidades
echo ""
echo "3️⃣ DESCOBERTA DE SITES"
discovery_data='{
  "cities": [
    {"name": "Muzambinho", "state": "MG"},
    {"name": "Alfenas", "state": "MG"},
    {"name": "Guaxupé", "state": "MG"}
  ],
  "options": {
    "max_sites_per_city": 10,
    "enable_validation": true,
    "search_engines": ["google", "directory", "domain_guessing"]
  }
}'

make_request "POST" "/cities/discover-sites" "$discovery_data" "Iniciar descoberta de sites para 3 cidades"

# Capturar job_id da resposta (simulado - em um teste real você extrairia da resposta JSON)
echo "💡 Dica: Copie o job_id da resposta acima para acompanhar o progresso"

# 4. Verificar jobs ativos
echo ""
echo "4️⃣ MONITORAMENTO DE JOBS"
make_request "GET" "/cities/discovery/jobs" "" "Listar jobs de descoberta ativos"

# 5. Aguardar um pouco e verificar novamente as cidades
echo ""
echo "⏳ Aguardando descoberta processar (30 segundos)..."
sleep 30

echo ""
echo "5️⃣ VERIFICAÇÃO APÓS DESCOBERTA"
make_request "GET" "/cities" "" "Listar cidades após descoberta"

# 6. Verificar sites de uma cidade específica
echo ""
echo "6️⃣ DETALHES DE CIDADE"
make_request "GET" "/cities/Muzambinho?state=MG" "" "Buscar detalhes da cidade Muzambinho"

# 7. Listar sites de uma cidade
make_request "GET" "/cities/Muzambinho/sites?state=MG&active=true" "" "Listar apenas sites ativos de Muzambinho"

# 8. Adicionar um site manualmente
echo ""
echo "7️⃣ ADIÇÃO MANUAL DE SITE"
manual_site_data='{
  "url": "https://www.imobiliariateste.com.br",
  "name": "Imobiliária Teste",
  "status": "active"
}'

make_request "POST" "/cities/Muzambinho/sites?state=MG" "$manual_site_data" "Adicionar site manualmente à Muzambinho"

# 9. Testar crawling com cidades específicas
echo ""
echo "8️⃣ TESTE DE CRAWLING"
crawler_data='{
  "cities": ["Muzambinho", "Alfenas"],
  "mode": "incremental"
}'

make_request "POST" "/crawler/trigger" "$crawler_data" "Executar crawling apenas para Muzambinho e Alfenas"

# 10. Verificar estatísticas
echo ""
echo "9️⃣ ESTATÍSTICAS DO SISTEMA"
make_request "GET" "/cities/statistics" "" "Obter estatísticas gerais do sistema"

# 11. Validar sites de uma cidade
echo ""
echo "🔟 VALIDAÇÃO DE SITES"
make_request "POST" "/cities/Muzambinho/validate?state=MG" "" "Validar sites da cidade Muzambinho"

echo ""
echo "✅ TESTES CONCLUÍDOS!"
echo "==================="
echo ""
echo "📝 PRÓXIMOS PASSOS MANUAIS:"
echo "1. Verificar logs do servidor para acompanhar o processamento"
echo "2. Testar outros endpoints conforme necessário"
echo "3. Verificar banco MongoDB para ver os dados salvos"
echo ""
echo "🔍 ENDPOINTS ADICIONAIS PARA TESTAR:"
echo "- GET /cities/discovery/jobs/{job_id} - Status de job específico"
echo "- DELETE /cities/Muzambinho?state=MG&confirm=true - Deletar cidade"
echo "- POST /cities/cleanup - Limpeza de sites inativos"
echo "- PUT /cities/Muzambinho/sites/{url}/stats - Atualizar estatísticas"
echo ""
echo "📊 MONITORAMENTO:"
echo "- Logs do servidor: tail -f logs/app.log"
echo "- MongoDB: mongo crawler --eval 'db.city_sites.find().pretty()'"

