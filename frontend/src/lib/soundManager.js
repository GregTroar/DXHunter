class SoundManager {
  constructor() {
    this.enabled = false;
    this.audioContext = null;
    this.sounds = {};
    this.init();
  }

  init() {
    // Initialize Web Audio API on user interaction
    if (typeof window !== 'undefined') {
      document.addEventListener('click', () => this.initAudioContext(), { once: true });
    }
  }

  initAudioContext() {
    if (!this.audioContext) {
      this.audioContext = new (window.AudioContext || window.webkitAudioContext)();
    }
  }

  // Generate a beep sound for watchlist alerts
  generateBeep(priority = 'medium') {
    if (!this.enabled || !this.audioContext) return;

    const ctx = this.audioContext;
    const oscillator = ctx.createOscillator();
    const gainNode = ctx.createGain();

    oscillator.connect(gainNode);
    gainNode.connect(ctx.destination);

    // Different frequencies for different priorities
    const frequencies = {
      high: 1200,   // High pitch
      medium: 800,  // Medium pitch
      low: 500      // Low pitch
    };

    oscillator.frequency.value = frequencies[priority] || frequencies.medium;
    oscillator.type = 'sine';

    // Volume and duration
    gainNode.gain.setValueAtTime(0.3, ctx.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 0.5);

    oscillator.start(ctx.currentTime);
    oscillator.stop(ctx.currentTime + 0.5);
  }

  // Play double beep for high priority
  playWatchlistAlert(priority = 'medium') {
    if (!this.enabled) return;

    this.initAudioContext();
    
    this.generateBeep(priority);
    
    if (priority === 'high') {
      // Double beep for high priority
      setTimeout(() => this.generateBeep(priority), 200);
    }
  }

  setEnabled(enabled) {
    this.enabled = enabled;
  }

  isEnabled() {
    return this.enabled;
  }
}

export const soundManager = new SoundManager();