<script>
  import { onMount, onDestroy } from 'svelte';
  import Header from './components/Header.svelte';
  import FilterBar from './components/FilterBar.svelte';
  import SpotsTable from './components/SpotsTable.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import Toast from './components/Toast.svelte';
  import ErrorBanner from './components/ErrorBanner.svelte';
  import { spotWorker } from './lib/spotWorker.js';
  import { spotCache } from './lib/spotCache.js';
  import LogsTab from './components/LogsTab.svelte';

  
  // State
  let spots = [];
  let filteredSpots = [];
  let stats = {
    totalSpots: 0,
    activeSpotters: 0,
    newDXCC: 0,
    connectedClients: 0,
    totalContacts: 0,
    clusterStatus: 'disconnected',
    clusterType: 'unknown',
    flexStatus: 'disconnected',
    myCallsign: '',
    filters: { skimmer: false, ft8: false, ft4: false }
  };
  let watchlist = []; // ✅ Initialisé vide, sera rempli par WebSocket
  let recentQSOs = [];
  let logStats = { today: 0, thisWeek: 0, thisMonth: 0, total: 0 };
  
  let dxccProgress = { worked: 0, total: 340, percentage: 0 };
  let activations = [];
  let solarData = { sfi: 'N/A', sunspots: 'N/A', aIndex: 'N/A', kIndex: 'N/A' };
  
  let activeTab = 'activations';
  let showOnlyActive = true;
  let showOnlyNotWorked = true;
  let wsStatus = 'disconnected';
  let errorMessage = '';
  let toastMessage = '';
  let toastType = 'info';
  let logs = [];

  let contestMode = false;
  let contestPrefix = "";
  let contestCallsigns = [];
  
  let spotFilters = {
    showAll: true,
    showNewDXCC: false,
    showNewBand: false,
    showNewMode: false,
    showNewBandMode: false,
    showNewSlot: false,
    showWorked: false,
    showWatchlist: false,
    showContest: false,
    showPOTA: false,
    showSOTA: false,
    showFT8: false,
    showFT4: false,
    showRTTY: false,
    showSSB: false,
    showCW: false,
    band160M: false,
    band80M: false,
    band60M: false,
    band40M: false,
    band30M: false,
    band20M: false,
    band17M: false,
    band15M: false,
    band12M: false,
    band10M: false,
    band6M: false
  };  
  
  // WebSocket
  let ws;
  let reconnectTimer;
  let reconnectAttempts = 0;
  let maxReconnectAttempts = 10;
  let isShuttingDown = false;
  let filterTimeout;
  let isFiltering = false;
  let cacheLoaded = false;
  let notifiedSpots = new Set();

  $: {
    if (spotFilters.showAll) {
      filteredSpots = spots;
      isFiltering = false;
      if (filterTimeout) {
        clearTimeout(filterTimeout);
        filterTimeout = null;
      }
    } else {
      if (filterTimeout) {
        clearTimeout(filterTimeout);
      }
      
      filterTimeout = setTimeout(async () => {
        isFiltering = true;
        try {
          // ✅ SI filtre Contest actif, ne PAS utiliser le worker
          if (spotFilters.showContest) {
            console.log("🏆 Using direct filter (Contest mode)");
            filteredSpots = applyFilters(spots, spotFilters, watchlist);
          } else {
            filteredSpots = await spotWorker.filterSpots(spots, spotFilters, watchlist);
          }
        } catch (error) {
          console.error('Filter error:', error);
          filteredSpots = spots;
        } finally {
          isFiltering = false;
          filterTimeout = null;
        }
      }, 150);
    }
  }
  
  $: if (watchlist.length > 0) {
    // Charger les spots de watchlist immédiatement
    setTimeout(() => {
      const event = new CustomEvent('loadWatchlistSpots');
      window.dispatchEvent(event);
    }, 100);
  }

function applyFilters(allSpots, filters, wl) {
  const bandFiltersActive = filters.band160M || filters.band80M || filters.band60M || 
    filters.band40M || filters.band30M || filters.band20M || filters.band17M || 
    filters.band15M || filters.band12M || filters.band10M || filters.band6M;
  
  const typeFiltersActive = filters.showNewDXCC || filters.showNewBand || 
    filters.showNewMode || filters.showNewBandMode || filters.showNewSlot || 
    filters.showWorked || filters.showWatchlist || filters.showContest ||
    filters.showPOTA || filters.showSOTA;
  
  const modeFiltersActive = filters.showFT8 || filters.showFT4 || filters.showRTTY || filters.showSSB || filters.showCW;
  
  return allSpots.filter(spot => {
    let matchesBand = false;
    let matchesType = false;
    let matchesMode = false;
    
    if (bandFiltersActive) {
      matchesBand = (
        (filters.band160M && spot.Band === '160M') ||
        (filters.band80M && spot.Band === '80M') ||
        (filters.band60M && spot.Band === '60M') ||
        (filters.band40M && spot.Band === '40M') ||
        (filters.band30M && spot.Band === '30M') ||
        (filters.band20M && spot.Band === '20M') ||
        (filters.band17M && spot.Band === '17M') ||
        (filters.band15M && spot.Band === '15M') ||
        (filters.band12M && spot.Band === '12M') ||
        (filters.band10M && spot.Band === '10M') ||
        (filters.band6M && spot.Band === '6M')
      );
    }
    
    if (typeFiltersActive) {
      // ✅ CORRECTION : Utiliser des IF séparés au lieu de ELSE IF
      
      if (filters.showContest) {
        // Vérifier si le spot match le contest
        if (contestPrefix && spot.DX.includes(contestPrefix)) {
          matchesType = true;
        } else if (contestCallsigns && contestCallsigns.length > 0) {
          const isContestCall = contestCallsigns.some(cc => 
            spot.DX === cc || spot.DX.startsWith(cc + '/')
          );
          if (isContestCall) matchesType = true;
        }
      }
      
      if (filters.showWatchlist) {
        const inWatchlist = wl.some(entry => 
          spot.DX === entry.callsign || spot.DX.startsWith(entry.callsign)
        );
        if (inWatchlist) matchesType = true;
      }
      
      if (filters.showNewDXCC && spot.NewDXCC) {
        matchesType = true;
      }
      
      if (filters.showNewBandMode && spot.NewBand && spot.NewMode && !spot.NewDXCC) {
        matchesType = true;
      }
      
      if (filters.showNewBand && spot.NewBand && !spot.NewMode && !spot.NewDXCC) {
        matchesType = true;
      }
      
      if (filters.showNewMode && spot.NewMode && !spot.NewBand && !spot.NewDXCC) {
        matchesType = true;
      }
      
      if (filters.showNewSlot && spot.NewSlot && !spot.NewDXCC && !spot.NewBand && !spot.NewMode) {
        matchesType = true;
      }
      
      if (filters.showWorked && spot.Worked) {
        matchesType = true;
      }
      if (filters.showPOTA && spot.POTARef) matchesType = true;
      if (filters.showSOTA && spot.SOTARef) matchesType = true;
    }
    
    if (modeFiltersActive) {
      const mode = spot.Mode || '';
      if (filters.showFT8 && mode === 'FT8') matchesMode = true;
      if (filters.showFT4 && mode === 'FT4') matchesMode = true;
      if (filters.showRTTY && mode === 'RTTY') matchesMode = true;
      if (filters.showSSB && ['SSB', 'USB', 'LSB'].includes(mode)) matchesMode = true;
      if (filters.showCW && mode === 'CW') matchesMode = true;
    }
    
    const numActiveFilterTypes = [bandFiltersActive, typeFiltersActive, modeFiltersActive].filter(Boolean).length;
    
    if (numActiveFilterTypes === 0) return false;
    if (numActiveFilterTypes === 1) {
      if (bandFiltersActive) return matchesBand;
      if (typeFiltersActive) return matchesType;
      if (modeFiltersActive) return matchesMode;
    }
    if (numActiveFilterTypes === 2) {
      if (bandFiltersActive && typeFiltersActive) return matchesBand && matchesType;
      if (bandFiltersActive && modeFiltersActive) return matchesBand && matchesMode;
      if (typeFiltersActive && modeFiltersActive) return matchesType && matchesMode;
    }
    if (numActiveFilterTypes === 3) {
      return matchesBand && matchesType && matchesMode;
    }
    
    return false;
  });
}
  
  function toggleFilter(filterName) {
    if (filterName === 'showAll') {
      spotFilters = {
        showAll: true,
        showNewDXCC: false,
        showNewBand: false,
        showNewMode: false,
        showNewBandMode: false,
        showNewSlot: false,
        showWorked: false,
        showWatchlist: false,
        showPOTA: false,
        showSOTA: false,
        showFT8: false,
        showFT4: false,
        showRTTY: false,
        showSSB: false,
        showCW: false,
        band160M: false,
        band80M: false,
        band60M: false,
        band40M: false,
        band30M: false,
        band20M: false,
        band17M: false,
        band15M: false,
        band12M: false,
        band10M: false,
        band6M: false
      };
    } else {
      spotFilters.showAll = false;
      spotFilters[filterName] = !spotFilters[filterName];
      
      const anyActive = Object.keys(spotFilters).some(key => 
        key !== 'showAll' && spotFilters[key]
      );
      if (!anyActive) {
        spotFilters.showAll = true;
      }
    }
  }
  
    function connectWebSocket() {
      if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
        return;
      }
      
      wsStatus = 'connecting';
      
      try {
        const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsHost = window.location.host; // Prend automatiquement l'IP:port depuis l'URL
        ws = new WebSocket(`${wsProtocol}//${wsHost}/api/ws`);
        
        ws.onopen = () => {
          console.log('WebSocket connected');
          wsStatus = 'connected';
          reconnectAttempts = 0;
          errorMessage = '';
          showToast('✅ Connected to DX Cluster', 'connection');
        };
        
        ws.onmessage = (event) => {
          try {
            const message = JSON.parse(event.data);
            handleWebSocketMessage(message);
          } catch (error) {
            console.error('Error parsing WebSocket message:', error);
          }
        };
        
        ws.onerror = (error) => {
          console.error('WebSocket error:', error);
          wsStatus = 'disconnected';
        };
        
        ws.onclose = () => {
          console.log('WebSocket closed');
          wsStatus = 'disconnected';
          
          if (isShuttingDown) {
            console.log('App is shutting down, skip reconnection');
            return;
          }
          
          if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
            errorMessage = `Connection lost. Reconnecting in ${delay/1000}s... (attempt ${reconnectAttempts}/${maxReconnectAttempts})`;
            
            reconnectTimer = setTimeout(() => {
              connectWebSocket();
            }, delay);
          } else {
            errorMessage = 'Unable to connect to server. Please refresh the page.';
          }
        };
      } catch (error) {
        console.error('Error creating WebSocket:', error);
        wsStatus = 'disconnected';
      } 
    }
  
  function handleWebSocketMessage(message) {

    switch (message.type) {
    case 'telnetCommandResponse':
      const { success, command, message: responseMsg } = message.data;
      if (success) {
        showToast(`✅ Command executed: ${command}`, 'success');
      } else {
        showToast(`❌ Failed: ${responseMsg}`, 'error');
      }
      break;
    case 'telnetResponse':
      // Dispatcher un event custom pour que ConsoleTab puisse l'écouter
      const telnetEvent = new CustomEvent('telnetResponse', {
        detail: message.data  // {message: "...", timestamp: "...", isCommand: bool}
      });
      window.dispatchEvent(telnetEvent);
      break;    
    case 'stats':
      stats = message.data;

      if (message.data.contestMode !== undefined) {
        contestMode = message.data.contestMode;
      }
      if (message.data.contestPrefix !== undefined) {
        contestPrefix = message.data.contestPrefix;
      }
      if (message.data.contestCallsigns !== undefined) {
        contestCallsigns = message.data.contestCallsigns || [];
      }
      break;
      case 'spots':
        const newSpots = message.data || [];
        
        // Détecter si votre indicatif a été spotté
        if (stats.myCallsign && newSpots.length > 0) {
          newSpots.forEach(spot => {
            if (spot.DX === stats.myCallsign && !notifiedSpots.has(spot.ID)) {
              notifiedSpots.add(spot.ID);
              showToast(
                `📢 You were spotted by ${spot.SpotterCallsign} on ${spot.FrequencyMhz} (${spot.Band} ${spot.Mode})`, 
                'mycall'
              );
            }
          });
          
          // ✅ Nettoyer les anciens IDs (garder seulement 200 derniers)
          if (notifiedSpots.size > 200) {
            const arr = Array.from(notifiedSpots);
            notifiedSpots = new Set(arr.slice(-200));
          }
        }
        
        spots = newSpots;
        
        // ✅ Debounce la sauvegarde du cache (toutes les 30 secondes max)
        if (spots.length > 0) {
          if (window.cacheSaveTimeout) clearTimeout(window.cacheSaveTimeout);
          window.cacheSaveTimeout = setTimeout(() => {
            spotCache.saveSpots(spots).catch(err => console.error('Cache save error:', err));
            window.cacheSaveTimeout = null; // ✅ Nettoyer la référence
          }, 30000); // 30 secondes
        }
        break;
      case 'watchlist':
        watchlist = message.data || [];
        spotCache.saveMetadata('watchlist', watchlist).catch(err => console.error('Cache save error:', err));
        break;
      case 'log':
        const previousQSOs = recentQSOs;
        recentQSOs = message.data || [];

        if (recentQSOs.length > 0) {
          spotCache.saveQSOs(recentQSOs).catch(err => console.error('Cache save error:', err));
        }

        const latestQSO = recentQSOs[0];
        const previousLatestQSO = previousQSOs[0];

        if (latestQSO && (!previousLatestQSO || 
            latestQSO.callsign !== previousLatestQSO.callsign ||
            latestQSO.band !== previousLatestQSO.band ||
            latestQSO.mode !== previousLatestQSO.mode)) {

          const newQSO = {
            ...latestQSO,
            band: latestQSO.band?.toUpperCase(),
            mode: latestQSO.mode?.toUpperCase(),
            dxcc: latestQSO.dxcc?.toString(),
          };

          // Fonction de recalcul partagée
          const recalcSpot = (spot) => {
            const sameCallBandMode = 
              spot.DX === newQSO.callsign && 
              spot.Band === newQSO.band && 
              spot.Mode === newQSO.mode;

            if (sameCallBandMode) {
              return { ...spot, Worked: true, NewBand: false, NewMode: false, NewDXCC: false, NewSlot: false };
            }

            if (spot.DXCC === newQSO.dxcc && spot.Band === newQSO.band && spot.Mode === newQSO.mode) {
              return { ...spot, NewDXCC: false, NewBand: false, NewMode: false, NewSlot: false };
            }

            if (spot.DXCC === newQSO.dxcc && spot.Band === newQSO.band) {
              return { ...spot, NewDXCC: false, NewBand: false };
            }

            if (spot.DXCC === newQSO.dxcc && spot.Mode === newQSO.mode) {
              return { ...spot, NewDXCC: false, NewMode: false };
            }

            if (spot.DXCC === newQSO.dxcc) {
              return { ...spot, NewDXCC: false };
            }

            return spot;
          };

          // ✅ Mettre à jour spots ET filteredSpots pour forcer le re-render
          spots = spots.map(recalcSpot);
          filteredSpots = filteredSpots.map(recalcSpot);
        }
        break;
      case 'appLog':
        // Un seul log applicatif
        if (message.data) {
          logs = [...logs, message.data];
          // Garder seulement les 500 derniers
          if (logs.length > 2000) {
            logs = logs.slice(-2000);
          }
        }
        break;
      case 'appLogs':
        // Logs initiaux (au chargement)
        logs = message.data || [];
        break;
      
      case 'dxccProgress':
        dxccProgress = message.data || { worked: 0, total: 340, percentage: 0 };
        break;
      case 'logStats':
        logStats = message.data || {};
        break;
      case 'adxo':
        const prevAdxo = activations;
        activations = message.data || [];
        if (prevAdxo.length === 0 && activations.length > 0) {
          const activeCount = activations.filter(a => a.status === 'active').length;
          showToast(`🌍 ${activations.length} DX activations loaded (${activeCount} active)`, 'success', 4000);
        }
        break;
      case 'milestone':
        const milestoneData = message.data;
        const toastType = milestoneData.type === 'qso' ? 'milestone' : 'band';
        showToast(milestoneData.message, toastType);
        break;
      case 'watchlistAlert':
        // Dispatch custom event for watchlist alert
        const alertEvent = new CustomEvent('watchlistAlert', {
          detail: message.data
        });
        window.dispatchEvent(alertEvent);
             
        // Show toast notification
        showToast(
          `🎯 ${message.data.callsign} spotted on ${message.data.band} ${message.data.mode}!`,
          'success'
        );
        break;
      // ✅ NOUVEAU : Gérer les changements de bande du FlexRadio
      case 'flexBandChange':
        // Dispatch custom event pour WatchlistTab
        const bandEvent = new CustomEvent('flexBandChange', {
          detail: message.data  // {frequency: 14.195, band: "20M"}
        });
        window.dispatchEvent(bandEvent);
        console.log(`FlexRadio band changed: ${message.data.band} (${message.data.frequency} MHz)`);
        break;
    }
  }
  
  function handleSendCommand(e) {
    const { command, clusterName } = e.detail;
    
    if (ws?.readyState !== WebSocket.OPEN) {
      showToast('❌ Not connected to server', 'error');
      return;
    }
    
    ws.send(JSON.stringify({
      type: 'telnetCommand',
      data: { command, clusterName: clusterName || '' }
    }));
    showToast(`📡 Sent: ${command}${clusterName ? ` → ${clusterName}` : ''}`, 'radio');
  }

  function showToast(message, type = 'info', duration = 5000) {
    toastMessage = message;
    toastType = type;
    setTimeout(() => {
      toastMessage = '';
    }, duration);
  }
  
  async function fetchSolarData() {
    try {
      const response = await fetch('/api/solar');
      const json = await response.json();
      if (json.success) {
        solarData = {
          sfi: json.data.sfi || 'N/A',
          sunspots: json.data.sunspots || 'N/A',
          aIndex: json.data.aIndex || 'N/A',
          kIndex: json.data.kIndex || 'N/A'
        };
      }
    } catch (error) {
      console.error('Error fetching solar data:', error);
    }
  }
  
  async function sendCallsign(callsign, frequency, mode) {
    try {
      const response = await fetch('/api/send-callsign', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ callsign, frequency, mode })
      });
      
      const data = await response.json();
      if (data.success) {
        showToast(`📻 Tuned to ${callsign} • ${frequency} • ${mode}`, 'radio');
      } else {
        showToast('❌ Failed to send to radio', 'error');
      }
    } catch (error) {
      console.error('Error sending callsign:', error);
      showToast(`❌ Connection error: ${error.message}`, 'error');
    }
  }
  
  async function updateClusterFilter(filterName, value) {
    try {
      const response = await fetch('/api/filters', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ [filterName]: value })
      });
      
      const data = await response.json();
      if (data.success) {
        stats.filters[filterName] = value;
        const filterLabel = filterName.toUpperCase();
        const status = value ? 'ON' : 'OFF';
        showToast(`🔧 ${filterLabel} filter ${status}`, 'success');
      }
    } catch (error) {
      console.error('Error updating filter:', error);
      showToast(`❌ Failed to update filter: ${error.message}`, 'error');
    }
  }
  
async function shutdownApp() {
  try {
    // ✅ Désactiver la reconnexion et masquer l'erreur IMMÉDIATEMENT
    isShuttingDown = true;
    errorMessage = '';
    maxReconnectAttempts = 0;
    
    // ✅ Fermer le WebSocket proprement
    if (ws) {
      ws.onclose = null; // Désactiver le handler de reconnexion
      ws.close();
    }
    if (reconnectTimer) clearTimeout(reconnectTimer);
    wsStatus = 'disconnected';
    
    showToast('⚡ Shutting down FlexDXCluster...', 'warning');
    
    // ✅ Envoyer la commande de shutdown au backend
    const response = await fetch('/api/shutdown', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    
    const data = await response.json();
    if (data.success) {
      // ✅ Afficher le message de shutdown après un court délai
      setTimeout(() => {
        document.body.innerHTML = `
          <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 text-white">
            <div class="text-center">
              <svg class="w-24 h-24 mx-auto mb-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <h1 class="text-4xl font-bold mb-4 bg-gradient-to-r from-blue-400 to-purple-400 bg-clip-text text-transparent">
                FlexDXCluster Stopped
              </h1>
              <p class="text-slate-400 text-lg">The application has been shut down successfully.</p>
              <p class="text-slate-500 text-sm mt-4">You can close this window.</p>
            </div>
          </div>
        `;
      }, 500);
    }
  } catch (error) {
    console.error('Error shutting down:', error);
    if (!isShuttingDown) {
      showToast(`❌ Shutdown failed: ${error.message}`, 'error');
    }
  }
}
  
  onMount(async () => {
    // ✅ Initialiser IndexedDB
    try {
      await spotCache.init();
      
      // ✅ Charger les données du cache immédiatement
      const cachedSpots = await spotCache.getSpots();
      if (cachedSpots.length > 0) {
        spots = cachedSpots;
        cacheLoaded = true;
        console.log('📦 Loaded data from cache');
      }

      window.addEventListener('loadWatchlistSpots', () => {
        if (watchlist.length > 0) {
          // Déclencher le chargement via un event dispatché vers WatchlistTab
          const event = new CustomEvent('fetchWatchlistSpots');
          window.dispatchEvent(event);
        }
      });
      
      // Charger watchlist du cache
      const cachedWatchlist = await spotCache.getMetadata('watchlist');
      if (cachedWatchlist) {
        watchlist = cachedWatchlist;
      }
      
      // Charger QSOs du cache
      const cachedQSOs = await spotCache.getQSOs();
      if (cachedQSOs.length > 0) {
        recentQSOs = cachedQSOs;
      }
      
    } catch (error) {
      console.error('Failed to initialize cache:', error);
    }
    
    // Initialiser le worker
    spotWorker.init();
    
    // Se connecter au WebSocket (qui va mettre à jour avec les données fraîches)
    connectWebSocket();
    fetchSolarData();

    const solarInterval = setInterval(fetchSolarData, 15 * 60 * 1000);

    const handleSendSpot = (e) => {
      sendCallsign(e.detail.callsign, e.detail.frequency, e.detail.mode);
    };

    const handleSendCommandEvent = (e) => {
      handleSendCommand(e);
    };

    window.addEventListener('sendSpot', handleSendSpot);
    window.addEventListener('sendCommand', handleSendCommandEvent);

    return () => {
      spotWorker.terminate();
      spotCache.close();
      if (ws) ws.close();
      if (reconnectTimer) clearTimeout(reconnectTimer);
      clearInterval(solarInterval);
      window.removeEventListener('sendSpot', handleSendSpot);
      window.removeEventListener('sendCommand', handleSendCommandEvent);
    };
  });

  onDestroy(() => {
    console.log('Cleaning up App...');
    
    // ✅ Nettoyer tous les timeouts
    if (filterTimeout) {
      clearTimeout(filterTimeout);
      filterTimeout = null;
    }
    
    if (window.cacheSaveTimeout) {
      clearTimeout(window.cacheSaveTimeout);
      window.cacheSaveTimeout = null;
    }
    
    notifiedSpots.clear();
    
    console.log('App cleanup complete');
  });

</script>

<div class="bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 text-white min-h-screen p-2">
  
  {#if errorMessage}
    <ErrorBanner message={errorMessage} on:close={() => errorMessage = ''} />
  {/if}
  
  {#if toastMessage}
    <Toast message={toastMessage} type={toastType} />
  {/if}
  
  <Header 
    {stats} 
    {solarData} 
    {wsStatus} 
    {cacheLoaded}
    on:shutdown={shutdownApp}
    on:ctyUpdate={(e) => showToast(e.detail.success ? `✅ CTY updated: ${e.detail.message}` : `❌ CTY update failed: ${e.detail.message}`, e.detail.success ? 'success' : 'error', 10000)}
  />
  
  <FilterBar 
    {spotFilters} 
    {spots}
    {watchlist}
    {isFiltering}
    {contestMode}
    {contestPrefix}
    {contestCallsigns}
    on:toggleFilter={(e) => toggleFilter(e.detail)} 
  />
  
  <div class="grid grid-cols-[2.8fr_1.2fr] gap-3 overflow-hidden" style="height: calc(100vh - 280px); min-height: 500px;">
    <div class="overflow-hidden">
      <SpotsTable
        spots={filteredSpots} 
        {watchlist}
        myCallsign={stats.myCallsign}
        on:clickSpot={(e) => sendCallsign(e.detail.callsign, e.detail.frequency, e.detail.mode)} 
      />
    </div>
    
    <div class="overflow-hidden">
      <Sidebar 
        bind:activeTab
        bind:showOnlyActive
        bind:showOnlyNotWorked
        {spots}
        {watchlist}
        {recentQSOs}
        {logStats}
        {dxccProgress}
        {activations}
        {logs}
        {contestMode}
        {contestPrefix}
        {contestCallsigns}
        wsStatus={wsStatus}
        clusterType={stats.clusterType || 'unknown'}
        clusters={stats.clusters || []}
        on:toast={(e) => showToast(e.detail.message, e.detail.type)}
        on:clearLogs={() => logs = []}
        on:sendCommand={handleSendCommand}
      />
    </div>
  </div>
</div>