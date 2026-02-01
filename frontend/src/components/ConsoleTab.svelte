<script>
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  
  export let wsStatus = 'disconnected';
  
  const dispatch = createEventDispatcher();
  
  let command = '';
  let commandHistory = [];
  let historyIndex = -1;
  let isSending = false;
  let consoleOutput = [];  // ← NOUVEAU: historique des réponses
  let consoleContainer;    // ← Référence pour auto-scroll
  let autoScroll = true;
  
  // Limite de lignes dans la console
  const MAX_CONSOLE_LINES = 500;
  
  // Commandes prédéfinies
const commonCommands = [
  { cmd: 'SET/NOFILTER', label: 'No Filter', desc: 'Remove all filters' },
  { cmd: 'SET/FILTER DOC/PASS 1A,3A,4O,9A,9H,C3,CT,CU,DL,E7,EA,EA6,EI,ER,ES,EU,F,G,GD,GI,GJ,GM,GU,GW,HA,HB,HB0,HV,I,IS,IT9,JW,JX,LA,LX,LY,LZ,OE,OH,OH0,OJ0,OK,OM,ON,OY,OZ,PA,S5,SM,SP,SV,SV5,SV9,T7,TA1,TF,TK,UA,UR,YL,YO,YU,Z6', label: 'EU Only', desc: 'Show Spots from Europe' },
  { cmd: 'SH/WWV', label: 'WWV', desc: 'Propagation Data' },
  { cmd: 'SH/WCY', label: 'WCY', desc: 'Geomagnetic Data' },
  { cmd: 'SET/SKIMMER', label: 'Skimmer ON', desc: 'Activate Skimmer' },
  { cmd: 'SET/NOSKIMMER', label: 'Skimmer OFF', desc: 'Deactivate Skimmer' },
  { cmd: 'SET/FT8', label: 'FT8 ON', desc: 'Activate FT8' },
  { cmd: 'SET/NOFT8', label: 'FT8 OFF', desc: 'Deactivate FT8' },
  { cmd: 'HELP', label: 'Help', desc: 'Help' }
];
  
  // ═══════════════════════════════════════════════════════════
  // NOUVEAU: Écouter les messages WebSocket
  // ═══════════════════════════════════════════════════════════
  function handleTelnetResponse(event) {
    const { message, timestamp, isCommand } = event.detail;
    
    addToConsole({
      text: message,
      time: timestamp,
      type: isCommand ? 'command' : 'response'
    });
  }
  
  function addToConsole(entry) {
    consoleOutput = [...consoleOutput, entry];
    
    // Limiter la taille
    if (consoleOutput.length > MAX_CONSOLE_LINES) {
      consoleOutput = consoleOutput.slice(-MAX_CONSOLE_LINES);
    }
    
    // Auto-scroll vers le bas
    if (autoScroll && consoleContainer) {
      setTimeout(() => {
        consoleContainer.scrollTop = consoleContainer.scrollHeight;
      }, 10);
    }
  }
  
  onMount(() => {
    window.addEventListener('telnetResponse', handleTelnetResponse);
  });
  
  onDestroy(() => {
    window.removeEventListener('telnetResponse', handleTelnetResponse);
  });
  
  function sendCommand() {
    if (!command.trim() || wsStatus !== 'connected' || isSending) return;
    
    const cmd = command.trim();
    isSending = true;
    
    // Ajouter à l'historique des commandes
    commandHistory = [cmd, ...commandHistory.filter(c => c !== cmd)];
    if (commandHistory.length > 50) commandHistory = commandHistory.slice(0, 50);
    historyIndex = -1;
    
    // Envoyer via WebSocket
    dispatch('sendCommand', { command: cmd });
    
    // Réinitialiser
    command = '';
    
    setTimeout(() => {
      isSending = false;
    }, 500);
  }
  
  function handleKeyDown(e) {
    switch(e.key) {
      case 'Enter':
        e.preventDefault();
        sendCommand();
        break;
        
      case 'ArrowUp':
        e.preventDefault();
        if (commandHistory.length > 0) {
          historyIndex = Math.min(historyIndex + 1, commandHistory.length - 1);
          command = commandHistory[historyIndex];
        }
        break;
        
      case 'ArrowDown':
        e.preventDefault();
        if (historyIndex > 0) {
          historyIndex--;
          command = commandHistory[historyIndex];
        } else if (historyIndex === 0) {
          historyIndex = -1;
          command = '';
        }
        break;
        
      case 'Tab':
        e.preventDefault();
        if (command) {
          const matches = commonCommands.filter(c => 
            c.cmd.toLowerCase().startsWith(command.toLowerCase())
          );
          if (matches.length === 1) {
            command = matches[0].cmd;
          }
        }
        break;
        
      case 'l':
        // Ctrl+L pour clear
        if (e.ctrlKey) {
          e.preventDefault();
          clearConsole();
        }
        break;
    }
  }
  
  function selectCommonCommand(cmd) {
    command = cmd;
    setTimeout(() => {
      const input = document.querySelector('#telnet-command-input');
      if (input) input.focus();
    }, 10);
  }
  
  function clearConsole() {
    consoleOutput = [];
  }
  
  function toggleAutoScroll() {
    autoScroll = !autoScroll;
  }
  
  // Détecter si l'utilisateur scroll manuellement
  function handleScroll() {
    if (!consoleContainer) return;
    const { scrollTop, scrollHeight, clientHeight } = consoleContainer;
    // Si on est proche du bas (< 50px), réactiver l'auto-scroll
    autoScroll = scrollHeight - scrollTop - clientHeight < 50;
  }
</script>

<div class="h-full flex flex-col overflow-hidden bg-slate-900/50 rounded-lg">
  <!-- Header -->
  <div class="p-3 border-b border-slate-700/50 flex-shrink-0">
    <div class="flex items-center justify-between mb-3">
      <h2 class="text-lg font-bold flex items-center gap-2">
        <svg class="w-5 h-5 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
        Telnet Console
      </h2>
      
      <div class="flex items-center gap-3">
        <!-- Status indicator -->
        <div class="flex items-center gap-1.5">
          <div class="w-2 h-2 rounded-full {wsStatus === 'connected' ? 'bg-green-500 animate-pulse' : 'bg-red-500'}"></div>
          <span class="text-xs {wsStatus === 'connected' ? 'text-green-400' : 'text-red-400'}">
            {wsStatus === 'connected' ? 'Connected' : 'Disconnected'}
          </span>
        </div>
        
        <!-- Auto-scroll toggle -->
        <button
          on:click={toggleAutoScroll}
          class="px-2 py-1 text-xs rounded transition-colors {autoScroll ? 'bg-blue-600 text-white' : 'bg-slate-700 text-slate-400'}"
          title="Toggle auto-scroll"
        >
          Auto-scroll {autoScroll ? 'ON' : 'OFF'}
        </button>
        
        <!-- Clear button -->
        <button
          on:click={clearConsole}
          class="px-2 py-1 text-xs bg-slate-700 hover:bg-slate-600 text-slate-300 rounded transition-colors"
          title="Clear console (Ctrl+L)"
        >
          Clear
        </button>
      </div>
    </div>
    
    <!-- Input de commande -->
    <div class="flex gap-2 mb-2">
      <div class="flex-1 relative">
        <span class="absolute left-3 top-1/2 transform -translate-y-1/2 text-green-400 font-mono">❯</span>
        <input
          id="telnet-command-input"
          type="text"
          bind:value={command}
          on:keydown={handleKeyDown}
          placeholder="Type command..."
          class="w-full pl-8 pr-16 py-2 bg-slate-800 border border-slate-600 rounded font-mono text-sm text-green-300 placeholder-slate-500 focus:outline-none focus:border-green-500 focus:ring-1 focus:ring-green-500/50"
          disabled={wsStatus !== 'connected' || isSending}
        />
        <div class="absolute right-2 top-1/2 transform -translate-y-1/2 text-slate-500 text-xs flex items-center gap-2">
          <span>↑↓</span>
          <span class="text-slate-600">|</span>
          <span>Enter ↵</span>
        </div>
      </div>
      <button
        on:click={sendCommand}
        class="px-4 py-2 bg-green-600 hover:bg-green-700 disabled:bg-slate-700 disabled:text-slate-500 rounded text-sm font-medium transition-colors"
        disabled={!command.trim() || wsStatus !== 'connected' || isSending}
      >
        {#if isSending}
          <svg class="w-4 h-4 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        {:else}
          Send
        {/if}
      </button>
    </div>
    
    <!-- Quick commands -->
    <div class="flex flex-wrap gap-1">
      {#each commonCommands as item}
        <button
          on:click={() => selectCommonCommand(item.cmd)}
          class="px-2 py-0.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-slate-200 rounded text-xs transition-colors border border-slate-700"
          title={item.desc}
          disabled={wsStatus !== 'connected'}
        >
          {item.label}
        </button>
      {/each}
    </div>
  </div>
  
  <!-- Console output -->
  <div 
    bind:this={consoleContainer}
    on:scroll={handleScroll}
    class="flex-1 overflow-y-auto p-3 font-mono text-sm bg-slate-950/50"
  >
    {#if consoleOutput.length === 0}
      <div class="text-center py-8 text-slate-500">
        <svg class="w-12 h-12 mx-auto mb-3 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
        </svg>
        <p>Console ready</p>
        <p class="text-xs mt-1">Type a command or select from quick commands above</p>
      </div>
    {:else}
      <div class="space-y-0.5">
        {#each consoleOutput as line, i}
          <div class="flex gap-2 hover:bg-slate-800/30 px-1 rounded {line.type === 'command' ? 'text-yellow-400' : 'text-slate-300'}">
            <span class="text-slate-600 text-xs w-16 flex-shrink-0">{line.time}</span>
            <span class="break-all whitespace-pre-wrap {line.type === 'command' ? 'font-bold' : ''}">{line.text}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
  
  <!-- Footer status -->
  <div class="px-3 py-1.5 border-t border-slate-700/50 flex items-center justify-between text-xs text-slate-500 bg-slate-900/30">
    <span>{consoleOutput.length} lines</span>
    <span class="flex items-center gap-2">
      <kbd class="px-1.5 py-0.5 bg-slate-800 rounded text-slate-400">Ctrl+L</kbd>
      <span>Clear</span>
      <span class="text-slate-600">|</span>
      <kbd class="px-1.5 py-0.5 bg-slate-800 rounded text-slate-400">Tab</kbd>
      <span>Autocomplete</span>
    </span>
  </div>
</div>