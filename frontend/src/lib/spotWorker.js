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
      
      this.callbacks.set(messageId, (filteredSpots) => {
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
      
      this.callbacks.set(messageId, (sortedSpots) => {
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