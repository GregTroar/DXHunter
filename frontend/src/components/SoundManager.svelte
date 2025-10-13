<script>
  import { onMount } from 'svelte';
  
  export let enabled = true;
  export let volume = 0.3;
  export let showControls = false; // Masquer les contrôles par défaut
  
  let audioContext;
  let isMuted = false;
  
  onMount(() => {
    audioContext = new (window.AudioContext || window.webkitAudioContext)();
    window.addEventListener('playSound', handlePlaySound);
    
    // Charger les préférences depuis localStorage
    const savedMuted = localStorage.getItem('soundMuted');
    const savedVolume = localStorage.getItem('soundVolume');
    
    if (savedMuted !== null) isMuted = savedMuted === 'true';
    if (savedVolume !== null) volume = parseFloat(savedVolume);
    
    return () => {
      window.removeEventListener('playSound', handlePlaySound);
      if (audioContext) audioContext.close();
    };
  });
  
  function handlePlaySound(event) {
    if (!enabled || isMuted) return;
    
    const { type } = event.detail;
    
    if (type === 'newDXCC') {
      playNewDXCCSound();
    } else if (type === 'watchlist') {
      playWatchlistSound();
    }
  }
  
  function playNewDXCCSound() {
    if (!audioContext) return;
    
    const now = audioContext.currentTime;
    const oscillator = audioContext.createOscillator();
    const gainNode = audioContext.createGain();
    
    oscillator.connect(gainNode);
    gainNode.connect(audioContext.destination);
    
    oscillator.type = 'sine';
    
    // Mélodie joyeuse : C5 -> E5 -> G5 -> C6
    oscillator.frequency.setValueAtTime(523.25, now);
    oscillator.frequency.setValueAtTime(659.25, now + 0.15);
    oscillator.frequency.setValueAtTime(783.99, now + 0.3);
    oscillator.frequency.setValueAtTime(1046.50, now + 0.45);
    
    gainNode.gain.setValueAtTime(0, now);
    gainNode.gain.linearRampToValueAtTime(volume, now + 0.05);
    gainNode.gain.setValueAtTime(volume * 0.8, now + 0.5);
    gainNode.gain.exponentialRampToValueAtTime(0.01, now + 0.7);
    
    oscillator.start(now);
    oscillator.stop(now + 0.7);
  }
  
  function playWatchlistSound() {
    if (!audioContext) return;
    
    const now = audioContext.currentTime;
    playBeep(now, 800, 0.1);
    playBeep(now + 0.15, 1000, 0.1);
  }
  
  function playBeep(startTime, frequency, duration) {
    const oscillator = audioContext.createOscillator();
    const gainNode = audioContext.createGain();
    
    oscillator.connect(gainNode);
    gainNode.connect(audioContext.destination);
    
    oscillator.type = 'sine';
    oscillator.frequency.setValueAtTime(frequency, startTime);
    
    gainNode.gain.setValueAtTime(0, startTime);
    gainNode.gain.linearRampToValueAtTime(volume, startTime + 0.01);
    gainNode.gain.exponentialRampToValueAtTime(0.01, startTime + duration);
    
    oscillator.start(startTime);
    oscillator.stop(startTime + duration);
  }
  
  function toggleMute() {
    isMuted = !isMuted;
    localStorage.setItem('soundMuted', isMuted.toString());
  }
  
  function updateVolume(newVolume) {
    volume = newVolume;
    localStorage.setItem('soundVolume', volume.toString());
  }
</script>

{#if showControls}
  <div class="fixed bottom-4 right-4 flex items-center gap-2 bg-slate-800/90 backdrop-blur rounded-lg border border-slate-700/50 p-2 shadow-lg z-50">
    <button 
      on:click={toggleMute}
      class="p-2 rounded transition-colors {isMuted ? 'bg-red-600/20 text-red-400 hover:bg-red-600/30' : 'bg-slate-700/50 text-slate-300 hover:bg-slate-700'}"
      title={isMuted ? 'Unmute sounds' : 'Mute sounds'}>
      {#if isMuted}
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" clip-rule="evenodd"></path>
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2"></path>
        </svg>
      {:else}
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"></path>
        </svg>
      {/if}
    </button>
    
    <div class="flex items-center gap-2 border-l border-slate-700 pl-2">
      <svg class="w-4 h-4 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.536 8.464a5 5 0 010 7.072"></path>
      </svg>
      <input 
        type="range" 
        min="0" 
        max="100" 
        value={volume * 100}
        on:input={(e) => updateVolume(e.target.value / 100)}
        class="w-20 h-1 bg-slate-700 rounded-lg appearance-none cursor-pointer accent-blue-500"
        title="Volume: {Math.round(volume * 100)}%"
      />
      <span class="text-xs text-slate-400 w-8">{Math.round(volume * 100)}%</span>
    </div>
  </div>
{/if}

<style>
  input[type="range"]::-webkit-slider-thumb {
    appearance: none;
    width: 12px;
    height: 12px;
    background: #3b82f6;
    border-radius: 50%;
    cursor: pointer;
  }
  
  input[type="range"]::-moz-range-thumb {
    width: 12px;
    height: 12px;
    background: #3b82f6;
    border-radius: 50%;
    cursor: pointer;
    border: none;
  }
</style>