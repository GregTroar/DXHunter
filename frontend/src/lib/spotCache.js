// spotCache.js - Gestionnaire de cache IndexedDB pour les spots

class SpotCache {
  constructor() {
    this.dbName = 'FlexDXClusterDB';
    this.version = 1;
    this.db = null;
  }

  async init() {
    return new Promise((resolve, reject) => {
      const request = indexedDB.open(this.dbName, this.version);

      request.onerror = () => {
        console.error('IndexedDB failed to open');
        reject(request.error);
      };

      request.onsuccess = () => {
        this.db = request.result;
        console.log('✅ IndexedDB initialized');
        resolve();
      };

      request.onupgradeneeded = (event) => {
        const db = event.target.result;

        // Store pour les spots
        if (!db.objectStoreNames.contains('spots')) {
          const spotStore = db.createObjectStore('spots', { keyPath: 'ID' });
          spotStore.createIndex('timestamp', 'timestamp', { unique: false });
          spotStore.createIndex('band', 'Band', { unique: false });
          console.log('Created spots store');
        }

        // Store pour les métadonnées (stats, watchlist, etc.)
        if (!db.objectStoreNames.contains('metadata')) {
          db.createObjectStore('metadata', { keyPath: 'key' });
          console.log('Created metadata store');
        }

        // Store pour les QSOs récents
        if (!db.objectStoreNames.contains('qsos')) {
          const qsoStore = db.createObjectStore('qsos', { keyPath: 'id', autoIncrement: true });
          qsoStore.createIndex('callsign', 'callsign', { unique: false });
          console.log('Created qsos store');
        }
      };
    });
  }

  // Sauvegarder les spots
  async saveSpots(spots) {
    if (!this.db || !spots || spots.length === 0) return;

    try {
      const transaction = this.db.transaction(['spots'], 'readwrite');
      const store = transaction.objectStore('spots');

      // ✅ Vider d'abord le store
      await store.clear();

      // ✅ Sauvegarder par batch de 100 pour éviter surcharge mémoire
      const timestamp = Date.now();
      const batchSize = 100;
      
      for (let i = 0; i < spots.length; i += batchSize) {
        const batch = spots.slice(i, i + batchSize);
        batch.forEach(spot => {
          store.put({ ...spot, timestamp });
        });
        
        // ✅ Petite pause pour libérer la mémoire
        if (i + batchSize < spots.length) {
          await new Promise(resolve => setTimeout(resolve, 0));
        }
      }

      await this.waitForTransaction(transaction);
      console.log(`✅ Saved ${spots.length} spots to cache`);
    } catch (error) {
      console.error('Error saving spots:', error);
    }
  }

  // Récupérer les spots du cache
  async getSpots(maxAge = 5 * 60 * 1000) { // 5 minutes par défaut
    if (!this.db) return [];

    try {
      const transaction = this.db.transaction(['spots'], 'readonly');
      const store = transaction.objectStore('spots');
      const request = store.getAll();

      const spots = await this.waitForRequest(request);

      // Vérifier l'âge du cache
      if (spots.length > 0) {
        const cacheAge = Date.now() - spots[0].timestamp;
        if (cacheAge > maxAge) {
          console.log('Cache too old, clearing...');
          await this.clearSpots();
          return [];
        }
      }

      console.log(`📦 Loaded ${spots.length} spots from cache`);
      return spots;
    } catch (error) {
      console.error('Error getting spots:', error);
      return [];
    }
  }

  // Supprimer les spots du cache
  async clearSpots() {
    if (!this.db) return;

    try {
      const transaction = this.db.transaction(['spots'], 'readwrite');
      const store = transaction.objectStore('spots');
      await store.clear();
      await this.waitForTransaction(transaction);
      console.log('🗑️ Cleared spots cache');
    } catch (error) {
      console.error('Error clearing spots:', error);
    }
  }

  // Sauvegarder les métadonnées (stats, watchlist, etc.)
  async saveMetadata(key, value) {
    if (!this.db) return;

    try {
      const transaction = this.db.transaction(['metadata'], 'readwrite');
      const store = transaction.objectStore('metadata');
      await store.put({ key, value, timestamp: Date.now() });
      await this.waitForTransaction(transaction);
    } catch (error) {
      console.error('Error saving metadata:', error);
    }
  }

  // Récupérer les métadonnées
  async getMetadata(key) {
    if (!this.db) return null;

    try {
      const transaction = this.db.transaction(['metadata'], 'readonly');
      const store = transaction.objectStore('metadata');
      const request = store.get(key);
      const result = await this.waitForRequest(request);
      return result ? result.value : null;
    } catch (error) {
      console.error('Error getting metadata:', error);
      return null;
    }
  }

  // Sauvegarder les QSOs récents
  async saveQSOs(qsos) {
    if (!this.db || !qsos || qsos.length === 0) return;

    try {
      const transaction = this.db.transaction(['qsos'], 'readwrite');
      const store = transaction.objectStore('qsos');

      // Vider d'abord
      await store.clear();

      // Ajouter les QSOs
      qsos.forEach((qso, index) => {
        store.put({ ...qso, id: index + 1 });
      });

      await this.waitForTransaction(transaction);
      console.log(`✅ Saved ${qsos.length} QSOs to cache`);
    } catch (error) {
      console.error('Error saving QSOs:', error);
    }
  }

  // Récupérer les QSOs du cache
  async getQSOs() {
    if (!this.db) return [];

    try {
      const transaction = this.db.transaction(['qsos'], 'readonly');
      const store = transaction.objectStore('qsos');
      const request = store.getAll();
      const qsos = await this.waitForRequest(request);
      console.log(`📦 Loaded ${qsos.length} QSOs from cache`);
      return qsos;
    } catch (error) {
      console.error('Error getting QSOs:', error);
      return [];
    }
  }

  // Utilitaires
  waitForRequest(request) {
    return new Promise((resolve, reject) => {
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
  }

  waitForTransaction(transaction) {
    return new Promise((resolve, reject) => {
      transaction.oncomplete = () => resolve();
      transaction.onerror = () => reject(transaction.error);
    });
  }

  // Fermer la base de données
  close() {
    if (this.db) {
      this.db.close();
      this.db = null;
      console.log('IndexedDB closed');
    }
  }

  // Supprimer complètement la base
  async deleteDatabase() {
    this.close();
    return new Promise((resolve, reject) => {
      const request = indexedDB.deleteDatabase(this.dbName);
      request.onsuccess = () => {
        console.log('🗑️ Database deleted');
        resolve();
      };
      request.onerror = () => reject(request.error);
    });
  }
}

// Export singleton
export const spotCache = new SpotCache();