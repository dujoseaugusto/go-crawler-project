# 🚀 Guia de Deploy - Menor Custo Possível

## 💰 **Análise de Custos (Projeto de Teste)**

### 🏆 **RECOMENDADO: Railway.app**
- **API**: $5/mês
- **Crawler Worker**: $5/mês  
- **MongoDB**: $0/mês (até 1GB)
- **Total**: **$10/mês**

### 🥈 **Alternativas**
| Plataforma | API | DB | Total/mês | Complexidade |
|------------|-----|----|-----------| -------------|
| Railway    | $5  | $0 | **$10**   | ⭐ Muito Fácil |
| Render     | $7  | $0 | **$14**   | ⭐⭐ Fácil |
| Cloud Run  | $5-15| $10| **$15-25**| ⭐⭐⭐ Médio |
| DigitalOcean| $12| $15| **$27**   | ⭐⭐⭐⭐ Difícil |

---

## 🚀 **Deploy no Railway (RECOMENDADO)**

### **Pré-requisitos**
- Conta no [Railway.app](https://railway.app)
- Conta no [MongoDB Atlas](https://cloud.mongodb.com) (gratuita)
- Conta no Google Cloud (para Gemini API)

### **Passo 1: Configurar MongoDB Atlas (GRÁTIS)**

1. Criar cluster gratuito no MongoDB Atlas
2. Obter string de conexão: `mongodb+srv://user:pass@cluster.mongodb.net/crawler`
3. Configurar IP whitelist: `0.0.0.0/0` (para Railway)

### **Passo 2: Configurar Gemini API**

1. Ir para [Google AI Studio](https://makersuite.google.com/app/apikey)
2. Criar API Key gratuita
3. Salvar a chave: `AIzaSyC...`

### **Passo 3: Deploy no Railway**

#### **3.1 Conectar Repositório**
```bash
# 1. Fazer push do código para GitHub
git add .
git commit -m "Prepare for Railway deployment"
git push origin main

# 2. No Railway.app:
# - New Project → Deploy from GitHub repo
# - Selecionar seu repositório
```

#### **3.2 Configurar Variáveis de Ambiente**
No Railway Dashboard, adicionar:

```env
# Database
MONGO_URI=mongodb+srv://user:pass@cluster.mongodb.net/crawler

# AI Service
GEMINI_API_KEY=AIzaSyC...

# App Config
PORT=8080
SITES_FILE=configs/sites.json

# Crawler Config (para worker)
APP_TYPE=api  # ou crawler para worker
CRAWLER_MODE=incremental
ENABLE_AI=true
ENABLE_FINGERPRINTING=true
MAX_AGE=24h
AI_THRESHOLD=6h
```

#### **3.3 Criar Dois Serviços**

**Serviço 1: API**
- Nome: `crawler-api`
- Variável: `APP_TYPE=api`
- Porta: `8080`

**Serviço 2: Worker**  
- Nome: `crawler-worker`
- Variável: `APP_TYPE=crawler`
- Sem porta exposta

### **Passo 4: Configurar Domínio (Opcional)**
- Railway fornece domínio gratuito: `https://seu-projeto.up.railway.app`
- Pode conectar domínio customizado

---

## 🔧 **Deploy Alternativo: Render.com**

### **Custos**: $14/mês ($7 API + $7 Worker)

```bash
# 1. Conectar GitHub ao Render
# 2. Criar Web Service (API)
# 3. Criar Background Worker (Crawler)
# 4. Usar PostgreSQL gratuito + adaptação do código
```

---

## ☁️ **Deploy Alternativo: Google Cloud Run**

### **Custos**: $5-25/mês (pay-per-use)

```bash
# 1. Configurar gcloud CLI
gcloud auth login
gcloud config set project SEU-PROJECT-ID

# 2. Build e push da imagem
docker build -t gcr.io/SEU-PROJECT-ID/go-crawler .
docker push gcr.io/SEU-PROJECT-ID/go-crawler

# 3. Deploy API
gcloud run deploy crawler-api \
  --image gcr.io/SEU-PROJECT-ID/go-crawler \
  --platform managed \
  --region us-central1 \
  --set-env-vars APP_TYPE=api \
  --allow-unauthenticated

# 4. Deploy Worker (Cloud Scheduler + Cloud Run)
gcloud run deploy crawler-worker \
  --image gcr.io/SEU-PROJECT-ID/go-crawler \
  --platform managed \
  --region us-central1 \
  --set-env-vars APP_TYPE=crawler \
  --no-allow-unauthenticated
```

---

## 📊 **Monitoramento e Otimização**

### **Logs e Métricas**
```bash
# Railway: Dashboard integrado
# Render: Dashboard + logs em tempo real
# Cloud Run: Google Cloud Console
```

### **Otimizações de Custo**
1. **Usar modo incremental**: 85-90% economia de recursos
2. **Configurar AI threshold**: Reduz custos de API
3. **Limitar concorrência**: `MaxConcurrency: 3-5`
4. **Cleanup automático**: Remove dados antigos

### **Escalabilidade**
- **Railway**: Auto-scaling até limites do plano
- **Render**: Auto-scaling configurável  
- **Cloud Run**: Auto-scaling completo (0 a milhares)

---

## 🚨 **Configurações de Segurança**

### **Variáveis de Ambiente Obrigatórias**
```env
MONGO_URI=mongodb+srv://...     # String de conexão MongoDB
GEMINI_API_KEY=AIzaSyC...       # Chave da API Gemini
PORT=8080                       # Porta da aplicação
```

### **Rate Limiting**
- API: 100 req/hora geral
- Crawler: 10 req/hora específico
- Configurado no middleware

### **Backup e Recuperação**
- MongoDB Atlas: Backup automático
- Código: Versionado no Git
- Configurações: Documentadas neste guia

---

## ✅ **Checklist de Deploy**

- [ ] MongoDB Atlas configurado
- [ ] Gemini API Key obtida
- [ ] Código commitado no GitHub
- [ ] Railway/Render projeto criado
- [ ] Variáveis de ambiente configuradas
- [ ] Deploy da API realizado
- [ ] Deploy do Worker realizado
- [ ] Health checks funcionando
- [ ] Logs sendo gerados
- [ ] Primeira execução testada

---

## 🆘 **Troubleshooting**

### **Problemas Comuns**

**1. Erro de Conexão MongoDB**
```bash
# Verificar IP whitelist no Atlas
# Verificar string de conexão
# Testar localmente primeiro
```

**2. API Gemini Não Funciona**
```bash
# Verificar quota da API
# Verificar chave válida
# Testar com curl
```

**3. Worker Não Executa**
```bash
# Verificar variável APP_TYPE=crawler
# Verificar logs do container
# Testar modo full primeiro
```

### **Comandos de Debug**
```bash
# Logs Railway
railway logs

# Logs Render  
render logs

# Logs Cloud Run
gcloud logging read "resource.type=cloud_run_revision"
```

---

## 💡 **Próximos Passos**

1. **Monitoramento**: Configurar alertas de erro
2. **Backup**: Agendar backup do MongoDB
3. **Domínio**: Configurar domínio personalizado
4. **SSL**: Verificar certificados (automático)
5. **Performance**: Monitorar métricas e otimizar

**Custo Final Estimado: $10-15/mês** para projeto completo em produção! 🎉
