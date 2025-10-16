// spotWorker.js - Wrapper pour gérer le Web Worker

class SpotWorkerManager {
  constructor() {
    this.worker = null;
    this.callbacks = new Map();
    this.messageId = 0;
  }
  
  init() {
    if (this.worker) return;
    
    try {
      this.worker = new Worker('/spot-worker.js');
      
      this.worker.onmessage = (e) => {
        const { type, data, messageId } = e.data;
        
        // Appeler le callback correspondant si présent
        if (messageId && this.callbacks.has(messageId)) {
          const callback = this.callbacks.get(messageId);
          callback(data);
          this.callbacks.delete(messageId);
        }
      };
      
      this.worker.onerror = (error) => {
        console.error('Worker error:', error);
      };
      
      console.log('✅ Spot Worker initialized');
    } catch (error) {
      console.error('Failed to initialize worker:', error);
    }
  }
  
  filterSpots(spots, filters, watchlist) {
    return new Promise((resolve) => {
      if (!this.worker) {
        console.warn('Worker not initialized, filtering on main thread');
        resolve(spots);
        return;
      }
      
      const messageId = ++this.messageId;
      
      // ✅ Créer un timeout pour éviter les callbacks orphelins
      const timeoutId = setTimeout(() => {
        if (this.callbacks.has(messageId)) {
          console.warn('Worker callback timeout, cleaning up');
          this.callbacks.delete(messageId);
          resolve(spots); // Fallback sur les spots non filtrés
        }
      }, 5000); // 5 secondes max
      
      this.callbacks.set(messageId, (filteredSpots) => {
        clearTimeout(timeoutId); // ✅ Nettoyer le timeout
        resolve(filteredSpots);
      });
      
      this.worker.postMessage({
        type: 'FILTER_SPOTS',
        messageId,
        data: { spots, filters, watchlist }
      });
    });
  }
  
  sortSpots(spots, sortBy = 'id', sortOrder = 'desc') {
    return new Promise((resolve) => {
      if (!this.worker) {
        console.warn('Worker not initialized, sorting on main thread');
        resolve(spots);
        return;
      }
      
      const messageId = ++this.messageId;
      
      // ✅ Timeout pour éviter les callbacks orphelins
      const timeoutId = setTimeout(() => {
        if (this.callbacks.has(messageId)) {
          console.warn('Worker callback timeout, cleaning up');
          this.callbacks.delete(messageId);
          resolve(spots);
        }
      }, 5000);
      
      this.callbacks.set(messageId, (sortedSpots) => {
        clearTimeout(timeoutId); // ✅ Nettoyer le timeout
        resolve(sortedSpots);
      });
      
      this.worker.postMessage({
        type: 'SORT_SPOTS',
        messageId,
        data: { spots, sortBy, sortOrder }
      });
    });
  }
  
  terminate() {
    if (this.worker) {
      this.worker.terminate();
      this.worker = null;
      this.callbacks.clear();
      console.log('Worker terminated');
    }
  }
}

// Export singleton
export const spotWorker = new SpotWorkerManager();