// Configuração da API
const API_BASE_URL = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1' 
    ? `http://localhost:${window.location.port || 8080}` 
    : 'https://go-crawler-project-production.up.railway.app';

// Estado da aplicação
let currentPage = 1;
let currentFilters = {};
let totalPages = 0;

// Inicialização
document.addEventListener('DOMContentLoaded', function() {
    initializeApp();
});

function initializeApp() {
    // Configurar event listeners
    document.getElementById('searchForm').addEventListener('submit', handleSearch);
    
    // Carregar dados iniciais (sem filtros)
    loadProperties();
    
    console.log('App inicializada');
}

// Função principal de busca
async function handleSearch(event) {
    event.preventDefault();
    currentPage = 1; // Reset para primeira página
    await loadProperties();
}

// Carregar propriedades da API
async function loadProperties(page = 1) {
    try {
        showLoading(true);
        
        // Coletar filtros do formulário
        const filters = collectFilters();
        currentFilters = filters;
        currentPage = page;
        
        // Construir URL da API
        const url = buildApiUrl(filters, page);
        console.log('Fazendo requisição para:', url);
        
        // Fazer requisição
        const response = await fetch(url);
        
        if (!response.ok) {
            throw new Error(`Erro na API: ${response.status} ${response.statusText}`);
        }
        
        const data = await response.json();
        console.log('Dados recebidos:', data);
        
        // Exibir resultados
        displayResults(data);
        updateSearchStatus(data);
        updatePagination(data);
        
    } catch (error) {
        console.error('Erro ao carregar propriedades:', error);
        showError('Erro ao carregar propriedades: ' + error.message);
        displayEmptyState('Erro ao carregar dados. Tente novamente.');
    } finally {
        showLoading(false);
    }
}

// Coletar filtros do formulário
function collectFilters() {
    const filters = {};
    
    // Busca geral
    const query = document.getElementById('query').value.trim();
    if (query) filters.q = query;
    
    // Localização
    const cidade = document.getElementById('cidade').value.trim();
    if (cidade) filters.cidade = cidade;
    
    const bairro = document.getElementById('bairro').value.trim();
    if (bairro) filters.bairro = bairro;
    
    // Tipo de imóvel
    const tipoImovel = document.getElementById('tipoImovel').value;
    if (tipoImovel) filters.tipo_imovel = tipoImovel;
    
    // Preço
    const valorMin = parseFloat(document.getElementById('valorMin').value);
    if (valorMin > 0) filters.valor_min = valorMin;
    
    const valorMax = parseFloat(document.getElementById('valorMax').value);
    if (valorMax > 0) filters.valor_max = valorMax;
    
    // Quartos
    const quartosMin = parseInt(document.getElementById('quartosMin').value);
    if (quartosMin > 0) filters.quartos_min = quartosMin;
    
    const quartosMax = parseInt(document.getElementById('quartosMax').value);
    if (quartosMax > 0) filters.quartos_max = quartosMax;
    
    // Banheiros
    const banheirosMin = parseInt(document.getElementById('banheirosMin').value);
    if (banheirosMin > 0) filters.banheiros_min = banheirosMin;
    
    const banheirosMax = parseInt(document.getElementById('banheirosMax').value);
    if (banheirosMax > 0) filters.banheiros_max = banheirosMax;
    
    // Área
    const areaMin = parseFloat(document.getElementById('areaMin').value);
    if (areaMin > 0) filters.area_min = areaMin;
    
    const areaMax = parseFloat(document.getElementById('areaMax').value);
    if (areaMax > 0) filters.area_max = areaMax;
    
    return filters;
}

// Construir URL da API
function buildApiUrl(filters, page = 1) {
    const params = new URLSearchParams();
    
    // Adicionar filtros
    Object.keys(filters).forEach(key => {
        params.append(key, filters[key]);
    });
    
    // Adicionar paginação
    params.append('page', page);
    params.append('page_size', 12); // 12 itens por página
    
    return `${API_BASE_URL}/properties/search?${params.toString()}`;
}

// Exibir resultados
function displayResults(data) {
    const resultsContainer = document.getElementById('results');
    
    if (!data.properties || data.properties.length === 0) {
        displayEmptyState('Nenhum imóvel encontrado com os filtros selecionados.');
        return;
    }
    
    const html = data.properties.map(property => createPropertyCard(property)).join('');
    resultsContainer.innerHTML = html;
    
    // Adicionar animação
    resultsContainer.classList.add('fade-in');
    setTimeout(() => resultsContainer.classList.remove('fade-in'), 500);
}

// Criar card de propriedade
function createPropertyCard(property) {
    // Formatação de valores
    const preco = formatPrice(property.valor || property.preco || 0);
    const titulo = property.titulo || property.descricao || 'Imóvel sem título';
    const endereco = property.endereco || '';
    const cidade = property.cidade || '';
    const bairro = property.bairro || '';
    const quartos = property.quartos || 0;
    const banheiros = property.banheiros || 0;
    const area = property.area_total || property.area || 0;
    
    // Localização completa
    const localizacao = [bairro, cidade].filter(Boolean).join(', ');
    
    return `
        <div class="col-lg-4 col-md-6 mb-4">
            <div class="card property-card h-100">
                <div class="property-image">
                    <i class="fas fa-home"></i>
                </div>
                <div class="card-body d-flex flex-column">
                    <div class="property-price mb-2">${preco}</div>
                    <h6 class="property-title">${titulo}</h6>
                    ${endereco ? `<p class="text-muted small mb-2">${endereco}</p>` : ''}
                    ${localizacao ? `<p class="property-location mb-3">${localizacao}</p>` : ''}
                    
                    <div class="property-features mt-auto">
                        ${quartos > 0 ? `
                            <div class="feature-item">
                                <i class="fas fa-bed"></i>
                                <span>${quartos} quarto${quartos > 1 ? 's' : ''}</span>
                            </div>
                        ` : ''}
                        ${banheiros > 0 ? `
                            <div class="feature-item">
                                <i class="fas fa-bath"></i>
                                <span>${banheiros} banheiro${banheiros > 1 ? 's' : ''}</span>
                            </div>
                        ` : ''}
                        ${area > 0 ? `
                            <div class="feature-item">
                                <i class="fas fa-ruler-combined"></i>
                                <span>${area}m²</span>
                            </div>
                        ` : ''}
                    </div>
                    
                    ${property.tipo_imovel ? `
                        <div class="mt-2">
                            <span class="badge bg-primary">${property.tipo_imovel}</span>
                        </div>
                    ` : ''}
                </div>
            </div>
        </div>
    `;
}

// Atualizar status da busca
function updateSearchStatus(data) {
    const statusElement = document.getElementById('searchStatus');
    const totalItems = data.total_items || 0;
    const currentPage = data.current_page || 1;
    const totalPages = data.total_pages || 0;
    
    if (totalItems === 0) {
        statusElement.textContent = 'Nenhum resultado encontrado';
        statusElement.className = 'text-muted';
    } else {
        const startItem = ((currentPage - 1) * data.page_size) + 1;
        const endItem = Math.min(currentPage * data.page_size, totalItems);
        
        statusElement.innerHTML = `
            Mostrando <strong>${startItem}-${endItem}</strong> de <strong>${totalItems}</strong> imóveis
            ${totalPages > 1 ? `(Página ${currentPage} de ${totalPages})` : ''}
        `;
        statusElement.className = 'text-muted';
    }
}

// Atualizar paginação
function updatePagination(data) {
    totalPages = data.total_pages || 0;
    const currentPage = data.current_page || 1;
    
    const topPagination = document.getElementById('topPagination');
    const bottomPagination = document.getElementById('bottomPagination');
    
    if (totalPages <= 1) {
        topPagination.innerHTML = '';
        bottomPagination.innerHTML = '';
        return;
    }
    
    const paginationHtml = createPaginationHtml(currentPage, totalPages);
    topPagination.innerHTML = paginationHtml;
    bottomPagination.innerHTML = paginationHtml;
}

// Criar HTML da paginação
function createPaginationHtml(currentPage, totalPages) {
    let html = '<nav><ul class="pagination">';
    
    // Botão anterior
    html += `
        <li class="page-item ${currentPage === 1 ? 'disabled' : ''}">
            <a class="page-link" href="#" onclick="changePage(${currentPage - 1})" aria-label="Anterior">
                <span aria-hidden="true">&laquo;</span>
            </a>
        </li>
    `;
    
    // Páginas
    const startPage = Math.max(1, currentPage - 2);
    const endPage = Math.min(totalPages, currentPage + 2);
    
    if (startPage > 1) {
        html += `<li class="page-item"><a class="page-link" href="#" onclick="changePage(1)">1</a></li>`;
        if (startPage > 2) {
            html += `<li class="page-item disabled"><span class="page-link">...</span></li>`;
        }
    }
    
    for (let i = startPage; i <= endPage; i++) {
        html += `
            <li class="page-item ${i === currentPage ? 'active' : ''}">
                <a class="page-link" href="#" onclick="changePage(${i})">${i}</a>
            </li>
        `;
    }
    
    if (endPage < totalPages) {
        if (endPage < totalPages - 1) {
            html += `<li class="page-item disabled"><span class="page-link">...</span></li>`;
        }
        html += `<li class="page-item"><a class="page-link" href="#" onclick="changePage(${totalPages})">${totalPages}</a></li>`;
    }
    
    // Botão próximo
    html += `
        <li class="page-item ${currentPage === totalPages ? 'disabled' : ''}">
            <a class="page-link" href="#" onclick="changePage(${currentPage + 1})" aria-label="Próximo">
                <span aria-hidden="true">&raquo;</span>
            </a>
        </li>
    `;
    
    html += '</ul></nav>';
    return html;
}

// Mudar página
function changePage(page) {
    if (page < 1 || page > totalPages || page === currentPage) return;
    
    loadProperties(page);
    
    // Scroll para o topo
    window.scrollTo({ top: 0, behavior: 'smooth' });
}

// Exibir estado vazio
function displayEmptyState(message) {
    const resultsContainer = document.getElementById('results');
    resultsContainer.innerHTML = `
        <div class="col-12">
            <div class="empty-state">
                <i class="fas fa-search"></i>
                <h4>${message}</h4>
                <p>Tente ajustar os filtros de busca ou limpar todos os filtros.</p>
            </div>
        </div>
    `;
}

// Mostrar/ocultar loading
function showLoading(show) {
    const loading = document.getElementById('loading');
    const results = document.getElementById('results');
    
    if (show) {
        loading.style.display = 'block';
        results.style.display = 'none';
    } else {
        loading.style.display = 'none';
        results.style.display = 'block';
    }
}

// Limpar filtros
function clearFilters() {
    document.getElementById('searchForm').reset();
    currentPage = 1;
    loadProperties();
    showToast('Filtros limpos com sucesso!');
}

// Trigger do crawler
async function triggerCrawler() {
    try {
        showToast('Iniciando atualização dos dados...', 'info');
        
        const response = await fetch(`${API_BASE_URL}/crawler/trigger`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });
        
        if (!response.ok) {
            throw new Error(`Erro: ${response.status}`);
        }
        
        const data = await response.json();
        showToast('Atualização iniciada! Os dados serão atualizados em breve.', 'success');
        
        // Recarregar dados após alguns segundos
        setTimeout(() => {
            loadProperties();
        }, 5000);
        
    } catch (error) {
        console.error('Erro ao trigger crawler:', error);
        showToast('Erro ao iniciar atualização: ' + error.message, 'error');
    }
}

// Mostrar toast
function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    const toastMessage = document.getElementById('toastMessage');
    
    toastMessage.textContent = message;
    
    // Atualizar classe do toast baseado no tipo
    toast.className = 'toast';
    if (type === 'success') {
        toast.classList.add('bg-success', 'text-white');
    } else if (type === 'error') {
        toast.classList.add('bg-danger', 'text-white');
    } else {
        toast.classList.add('bg-info', 'text-white');
    }
    
    const bsToast = new bootstrap.Toast(toast);
    bsToast.show();
}

// Mostrar erro
function showError(message) {
    showToast(message, 'error');
}

// Formatação de preço
function formatPrice(price) {
    if (!price || price === 0) return 'Preço sob consulta';
    
    return new Intl.NumberFormat('pt-BR', {
        style: 'currency',
        currency: 'BRL',
        minimumFractionDigits: 0,
        maximumFractionDigits: 0
    }).format(price);
}

// Utilitários
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}
