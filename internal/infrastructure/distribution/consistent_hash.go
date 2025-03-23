package distribution

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

// ConsistentHash implementa el algoritmo de hashing consistente
// que permite distribuir carga entre múltiples nodos de forma equilibrada
// y minimizando redistribuciones cuando cambia el número de nodos
type ConsistentHash struct {
	// Número de réplicas virtuales por nodo real
	replicas int
	// Anillo de hash ordenado
	ring []uint32
	// Mapeo de hash a nodos
	hashMap map[uint32]string
	// Sincronización para operaciones concurrentes
	mu sync.RWMutex
	// Función de hash personalizada (opcional)
	hashFunc func(data []byte) uint32
}

// NewConsistentHash crea una nueva instancia de ConsistentHash
func NewConsistentHash(replicas int, hashFunc func(data []byte) uint32) *ConsistentHash {
	ch := &ConsistentHash{
		replicas: replicas,
		ring:     []uint32{},
		hashMap:  make(map[uint32]string),
	}

	if hashFunc == nil {
		ch.hashFunc = crc32.ChecksumIEEE
	} else {
		ch.hashFunc = hashFunc
	}

	return ch
}

// Add agrega un nodo al anillo de hash
func (ch *ConsistentHash) Add(node string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// Agregar replicas virtuales para cada nodo real
	for i := 0; i < ch.replicas; i++ {
		key := []byte(node + strconv.Itoa(i))
		hash := ch.hashFunc(key)
		ch.ring = append(ch.ring, hash)
		ch.hashMap[hash] = node
	}

	// Ordenar el anillo
	sort.Slice(ch.ring, func(i, j int) bool {
		return ch.ring[i] < ch.ring[j]
	})
}

// Remove elimina un nodo del anillo de hash
func (ch *ConsistentHash) Remove(node string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// Eliminar todas las réplicas del nodo
	for i := 0; i < ch.replicas; i++ {
		key := []byte(node + strconv.Itoa(i))
		hash := ch.hashFunc(key)

		// Eliminar del mapa
		delete(ch.hashMap, hash)
	}

	// Reconstruir el anillo
	var newRing []uint32
	for _, hash := range ch.ring {
		if _, ok := ch.hashMap[hash]; ok {
			newRing = append(newRing, hash)
		}
	}
	ch.ring = newRing
}

// Get devuelve el nodo responsable de la clave proporcionada
func (ch *ConsistentHash) Get(key string) string {
	if len(ch.ring) == 0 {
		return ""
	}

	ch.mu.RLock()
	defer ch.mu.RUnlock()

	// Calcular hash de la clave
	hash := ch.hashFunc([]byte(key))

	// Buscar el siguiente punto en el anillo
	idx := ch.search(hash)
	return ch.hashMap[ch.ring[idx]]
}

// search busca el índice apropiado en el anillo usando búsqueda binaria
func (ch *ConsistentHash) search(hash uint32) int {
	ringLen := len(ch.ring)

	// Búsqueda binaria
	idx := sort.Search(ringLen, func(i int) bool {
		return ch.ring[i] >= hash
	})

	// Si no se encontró un punto mayor o igual, volver al principio del anillo
	if idx == ringLen {
		idx = 0
	}

	return idx
}

// GetNodes devuelve todos los nodos actualmente en el anillo
func (ch *ConsistentHash) GetNodes() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	// Usar un mapa para eliminar duplicados
	nodes := make(map[string]struct{})
	for _, node := range ch.hashMap {
		nodes[node] = struct{}{}
	}

	// Convertir a slice
	result := make([]string, 0, len(nodes))
	for node := range nodes {
		result = append(result, node)
	}

	return result
}

// GetMultiple devuelve múltiples nodos para una clave
// útil para replicación o redundancia
func (ch *ConsistentHash) GetMultiple(key string, count int) []string {
	if len(ch.ring) == 0 {
		return []string{}
	}

	ch.mu.RLock()
	defer ch.mu.RUnlock()

	// No podemos devolver más nodos de los que hay
	uniqueNodes := len(ch.GetNodes())
	if count > uniqueNodes {
		count = uniqueNodes
	}

	// Calcular hash de la clave
	hash := ch.hashFunc([]byte(key))

	// Buscar el primer nodo
	idx := ch.search(hash)

	// Obtener nodos únicos
	result := make([]string, 0, count)
	seen := make(map[string]struct{})

	// Recorrer el anillo hasta encontrar suficientes nodos únicos
	for len(result) < count {
		node := ch.hashMap[ch.ring[idx]]
		if _, exists := seen[node]; !exists {
			seen[node] = struct{}{}
			result = append(result, node)
		}

		idx = (idx + 1) % len(ch.ring)
	}

	return result
}
