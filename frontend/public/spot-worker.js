// spot-worker.js - Web Worker pour traiter les spots

let lastProcessedData = null;

self.onmessage = function(e) {
  const { type, data, messageId } = e.data;
  
  // Libérer l'ancienne référence
  lastProcessedData = null;
  
  switch(type) {
    case 'FILTER_SPOTS':
      const filtered = filterSpots(data.spots, data.filters, data.watchlist);
      self.postMessage({ type: 'FILTERED_SPOTS', data: filtered, messageId });
      lastProcessedData = null;
      break;
      
    case 'SORT_SPOTS':
      const sorted = sortSpots(data.spots, data.sortBy, data.sortOrder);
      self.postMessage({ type: 'SORTED_SPOTS', data: sorted, messageId });
      lastProcessedData = null;
      break;
      
    default:
      console.error('Unknown worker message type:', type);
  }
};

function filterSpots(allSpots, filters, watchlist) {
  if (!allSpots || !Array.isArray(allSpots)) return [];
  
  // Si "All" est actif, retourner tous les spots
  if (filters.showAll) {
    return allSpots;
  }
  
  const bandFiltersActive = filters.band160M || filters.band80M || filters.band60M || 
    filters.band40M || filters.band30M || filters.band20M || filters.band17M || 
    filters.band15M || filters.band12M || filters.band10M || filters.band6M;
  
  const typeFiltersActive = filters.showNewDXCC || filters.showNewBand || 
    filters.showNewMode || filters.showNewBandMode || filters.showNewSlot || 
    filters.showWorked || filters.showWatchlist || filters.showPOTA || filters.showSOTA;
  
  const modeFiltersActive = filters.showFT8 || filters.showFT4 || filters.showRTTY || filters.showSSB || filters.showCW;
  
  return allSpots.filter(spot => {
    let matchesBand = false;
    let matchesType = false;
    let matchesMode = false;
    
    // Filtres de bande
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
    
    // Filtres de type
    if (typeFiltersActive) {
      if (filters.showWatchlist) {
        const inWatchlist = watchlist.some(entry => 
          spot.DX === entry.callsign || spot.DX.startsWith(entry.callsign)
        );
        if (inWatchlist) matchesType = true;
      }
      if (filters.showNewDXCC && spot.NewDXCC) matchesType = true;
      else if (filters.showNewBandMode && spot.NewBand && spot.NewMode && !spot.NewDXCC) matchesType = true;
      else if (filters.showNewBand && spot.NewBand && !spot.NewMode && !spot.NewDXCC) matchesType = true;
      else if (filters.showNewMode && spot.NewMode && !spot.NewBand && !spot.NewDXCC) matchesType = true;
      else if (filters.showNewSlot && spot.NewSlot && !spot.NewDXCC && !spot.NewBand && !spot.NewMode) matchesType = true;
      else if (filters.showWorked && spot.Worked) matchesType = true;
      if (filters.showPOTA && spot.POTARef) matchesType = true;
      if (filters.showSOTA && spot.SOTARef) matchesType = true;
    }
    
    // Filtres de mode
    if (modeFiltersActive) {
      const mode = spot.Mode || '';
      if (filters.showFT8 && mode === 'FT8') matchesMode = true;
      if (filters.showFT4 && mode === 'FT4') matchesMode = true;
      if (filters.showRTTY && mode === 'RTTY') matchesMode = true;
      if (filters.showSSB && ['SSB', 'USB', 'LSB'].includes(mode)) matchesMode = true;
      if (filters.showCW && mode === 'CW') matchesMode = true;
    }
    
    // Logique de combinaison des filtres
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

function sortSpots(spots, sortBy, sortOrder) {
  if (!spots || !Array.isArray(spots)) return [];
  
  const sorted = [...spots].sort((a, b) => {
    let compareValue = 0;
    
    switch(sortBy) {
      case 'dx':
        compareValue = a.DX.localeCompare(b.DX);
        break;
      case 'frequency':
        compareValue = parseFloat(a.FrequencyMhz) - parseFloat(b.FrequencyMhz);
        break;
      case 'band':
        compareValue = a.Band.localeCompare(b.Band);
        break;
      case 'mode':
        compareValue = a.Mode.localeCompare(b.Mode);
        break;
      case 'time':
        compareValue = a.UTCTime.localeCompare(b.UTCTime);
        break;
      default:
        compareValue = a.ID - b.ID; // Par défaut, tri par ID
    }
    
    return sortOrder === 'asc' ? compareValue : -compareValue;
  });
  
  return sorted;
}

console.log('Spot Worker initialized');