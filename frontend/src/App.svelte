<script>
  import { onMount, onDestroy } from 'svelte';
  import Header from './components/Header.svelte';
  import StatsCards from './components/StatsCards.svelte';
  import FilterBar from './components/FilterBar.svelte';
  import SpotsTable from './components/SpotsTable.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import Toast from './components/Toast.svelte';
  import ErrorBanner from './components/ErrorBanner.svelte';
  
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
    flexStatus: 'disconnected',
    myCallsign: '',
    filters: { skimmer: false, ft8: false, ft4: false }
  };
  let topSpotters = [];
  let watchlist = [];
  let recentQSOs = [];
  let logStats = { today: 0, thisWeek: 0, thisMonth: 0, total: 0 };
  let dxccProgress = { worked: 0, total: 340, percentage: 0 };
  let solarData = { sfi: 'N/A', sunspots: 'N/A', aIndex: 'N/A', kIndex: 'N/A' };
  
  let activeTab = 'stats';
  let wsStatus = 'disconnected';
  let errorMessage = '';
  let toastMessage = '';
  let toastType = 'info';
  
  let spotFilters = {
    showAll: true,
    showNewDXCC: false,
    showNewBand: false,
    showNewMode: false,
    showNewBandMode: false,
    showNewSlot: false,
    showWorked: false,
    showWatchlist: false,
    showDigital: false,
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
  
  // Reactive filtered spots
  $: {
    if (spotFilters.showAll) {
      filteredSpots = spots;
    } else {
      filteredSpots = applyFilters(spots, spotFilters, watchlist);
    }
  }
  
  function applyFilters(allSpots, filters, wl) {
    const bandFiltersActive = filters.band160M || filters.band80M || filters.band60M || 
      filters.band40M || filters.band30M || filters.band20M || filters.band17M || 
      filters.band15M || filters.band12M || filters.band10M || filters.band6M;
    
    const typeFiltersActive = filters.showNewDXCC || filters.showNewBand || 
      filters.showNewMode || filters.showNewBandMode || filters.showNewSlot || 
      filters.showWorked || filters.showWatchlist;
    
    const modeFiltersActive = filters.showDigital || filters.showSSB || filters.showCW;
    
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
        if (filters.showWatchlist) {
          const inWatchlist = wl.some(pattern => 
            spot.DX === pattern || spot.DX.startsWith(pattern)
          );
          if (inWatchlist) matchesType = true;
        }
        if (filters.showNewDXCC && spot.NewDXCC) matchesType = true;
        else if (filters.showNewBandMode && spot.NewBand && spot.NewMode && !spot.NewDXCC) matchesType = true;
        else if (filters.showNewBand && spot.NewBand && !spot.NewMode && !spot.NewDXCC) matchesType = true;
        else if (filters.showNewMode && spot.NewMode && !spot.NewBand && !spot.NewDXCC) matchesType = true;
        else if (filters.showNewSlot && spot.NewSlot && !spot.NewDXCC && !spot.NewBand && !spot.NewMode) matchesType = true;
        else if (filters.showWorked && spot.Worked) matchesType = true;
      }
      
      if (modeFiltersActive) {
        const mode = spot.Mode || '';
        if (filters.showDigital && ['FT8', 'FT4', 'RTTY'].includes(mode)) matchesMode = true;
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
        showDigital: false,
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
      ws = new WebSocket('ws://localhost:8080/api/ws');
      
      ws.onopen = () => {
        console.log('WebSocket connected');
        wsStatus = 'connected';
        reconnectAttempts = 0;
        errorMessage = '';
        showToast('Connected to server', 'success');
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
      case 'stats':
        stats = message.data;
        break;
      case 'spots':
        spots = message.data || [];
        break;
      case 'spotters':
        topSpotters = message.data || [];
        break;
      case 'watchlist':
        watchlist = message.data || [];
        break;
      case 'log':
        recentQSOs = message.data || [];
        break;
      case 'logStats':
        logStats = message.data || {};
        break;
      case 'dxccProgress':
        dxccProgress = message.data || { worked: 0, total: 340, percentage: 0 };
        break;
    }
  }
  
  function showToast(message, type = 'info') {
    toastMessage = message;
    toastType = type;
    setTimeout(() => {
      toastMessage = '';
    }, 3000);
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
        showToast(`${callsign} Sent - Radio tuned on ${frequency} in ${mode}`, 'success');
      } else {
        showToast('Failed to send', 'error');
      }
    } catch (error) {
      console.error('Error sending callsign:', error);
      showToast(`Error: ${error.message}`, 'error');
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
        showToast(`Filter ${filterName} updated`, 'success');
      }
    } catch (error) {
      console.error('Error updating filter:', error);
      showToast(`Update error: ${error.message}`, 'error');
    }
  }
  
  async function shutdownApp() {
    try {
      const response = await fetch('/api/shutdown', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });
      
      const data = await response.json();
      if (data.success) {
        showToast('FlexDXCluster shutting down...', 'info');
        
        // Fermer le WebSocket et arrêter les tentatives de reconnexion
        if (ws) ws.close();
        if (reconnectTimer) clearTimeout(reconnectTimer);
        wsStatus = 'disconnected';
        maxReconnectAttempts = 0; // Empêcher les reconnexions
        
        // Afficher la page de shutdown après 1 seconde
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
        }, 1000);
      }
    } catch (error) {
      console.error('Error shutting down:', error);
      showToast(`Cannot shutdown: ${error.message}`, 'error');
    }
  }
  
  onMount(() => {
    connectWebSocket();
    fetchSolarData();
    
    // Update solar data every 15 minutes
    const solarInterval = setInterval(fetchSolarData, 15 * 60 * 1000);
    
    // Listen for sendSpot events from watchlist
    const handleSendSpot = (e) => {
      sendCallsign(e.detail.callsign, e.detail.frequency, e.detail.mode);
    };
    window.addEventListener('sendSpot', handleSendSpot);
    
    return () => {
      if (ws) ws.close();
      if (reconnectTimer) clearTimeout(reconnectTimer);
      clearInterval(solarInterval);
      window.removeEventListener('sendSpot', handleSendSpot);
    };
  });
</script>

<div class="bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 text-white min-h-screen p-4">
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
    on:shutdown={shutdownApp} 
  />
  
  <StatsCards 
    {stats} 
    on:filterChange={(e) => updateClusterFilter(e.detail.name, e.detail.value)} 
  />
  
  <FilterBar 
    {spotFilters} 
    {spots}
    {watchlist}
    on:toggleFilter={(e) => toggleFilter(e.detail)} 
  />
  
  <div class="grid grid-cols-4 gap-3" style="height: calc(100vh - 360px);">
    <div class="col-span-3">
      <SpotsTable 
        spots={filteredSpots} 
        {watchlist}
        myCallsign={stats.myCallsign}
        on:clickSpot={(e) => sendCallsign(e.detail.callsign, e.detail.frequency, e.detail.mode)} 
      />
    </div>
    
    <Sidebar 
      bind:activeTab
      {topSpotters}
      {spots}
      {watchlist}
      {recentQSOs}
      {logStats}
      {dxccProgress}
      on:toast={(e) => showToast(e.detail.message, e.detail.type)}
    />
  </div>
</div>