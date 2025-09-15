# 🐳 **Guia Docker - Go Crawler Project**

## 📁 **Arquivos Docker Disponíveis**

### **1. `docker-compose.yaml` (PRINCIPAL)**
**Uso**: Desenvolvimento local completo
**Comando**: `docker-compose up`

**Serviços incluídos**:
- 🌐 **API + Interface Web** (porta 8080)
- 🤖 **Crawler Worker** (background)
- 🗄️ **MongoDB** (porta 27017)

**Acesso**:
- Interface Web: http://localhost:8080
- API: http://localhost:8080/properties
- Health Check: http://localhost:8080/health

---

### **2. `docker-compose.testing.yaml` (TESTES)**
**Uso**: Testes específicos do sistema incremental
**Comando**: `docker-compose -f docker-compose.testing.yaml --profile PROFILE up`

**Perfis disponíveis**:
- `full` - Crawling completo (primeira execução)
- `incremental` - Crawling incremental (demonstra economia)
- `fast` - Crawling sem IA (máxima velocidade)
- `stats` - Mostrar estatísticas do sistema

**Exemplos**:
```bash
# Teste crawling completo
docker-compose -f docker-compose.testing.yaml --profile full up

# Teste crawling incremental
docker-compose -f docker-compose.testing.yaml --profile incremental up

# Ver estatísticas
docker-compose -f docker-compose.testing.yaml --profile stats up
```

---

## 🚀 **Comandos Principais**

### **Desenvolvimento Normal**
```bash
# Iniciar tudo (recomendado)
docker-compose up

# Iniciar em background
docker-compose up -d

# Ver logs
docker-compose logs -f

# Parar tudo
docker-compose down
```

### **Rebuild após mudanças**
```bash
# Rebuild e iniciar
docker-compose up --build

# Rebuild apenas um serviço
docker-compose build api
docker-compose up api
```

### **Limpeza**
```bash
# Parar e remover volumes
docker-compose down -v

# Limpar tudo (cuidado!)
docker system prune -a
```

---

## 🔧 **Configuração**

### **Arquivo .env necessário**
Crie um arquivo `.env` na raiz com:

```env
# MongoDB (use Atlas para produção)
MONGO_URI=mongodb://db:27017/crawler

# Gemini API (opcional para testes)
GEMINI_API_KEY=sua_chave_aqui

# Configurações
PORT=8080
SITES_FILE=configs/sites.json
```

### **Para usar MongoDB Atlas**
```env
MONGO_URI=mongodb+srv://user:pass@cluster.mongodb.net/crawler
```

---

## 📊 **Monitoramento**

### **Health Checks**
- API: http://localhost:8080/health
- MongoDB: Automático via Docker

### **Logs Úteis**
```bash
# Logs da API
docker-compose logs -f api

# Logs do Crawler
docker-compose logs -f crawler

# Logs do MongoDB
docker-compose logs -f db
```

---

## 🐛 **Troubleshooting**

### **Problemas Comuns**

**1. Porta 8080 em uso**
```bash
# Ver o que está usando a porta
lsof -i :8080

# Ou mudar a porta no docker-compose.yaml
ports:
  - "8081:8080"
```

**2. MongoDB não conecta**
```bash
# Verificar se o container está rodando
docker-compose ps

# Verificar logs do MongoDB
docker-compose logs db
```

**3. Rebuild necessário**
```bash
# Após mudanças no código
docker-compose down
docker-compose up --build
```

### **Reset Completo**
```bash
# Parar tudo e limpar
docker-compose down -v
docker system prune -f
docker-compose up --build
```

---

## 📝 **Notas Importantes**

- **Arquivo principal**: Use sempre `docker-compose.yaml`
- **Testes**: Use `docker-compose.testing.yaml` apenas para testes específicos
- **Produção**: Use Railway.app (não Docker Compose)
- **Dados**: MongoDB data persiste em volume `mongo_data`
- **Interface Web**: Incluída automaticamente na API
