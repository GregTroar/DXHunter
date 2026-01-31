<script>
  import { createEventDispatcher } from 'svelte';
  
  export let wsStatus = 'disconnected';
  
  const dispatch = createEventDispatcher();
  
  let command = '';
  let commandHistory = [];
  let historyIndex = -1;
  let isSending = false;
  
  // Exemple de commandes prédéfinies
  const commonCommands = [
    { cmd: 'SH/DX', desc: 'Afficher les spots récents' },
    { cmd: 'SH/DX 20', desc: 'Afficher 20 spots' },
    { cmd: 'SH/DX/WW', desc: 'Spots mondiaux' },
    { cmd: 'SH/DX/EU', desc: 'Spots Europe' },
    { cmd: 'SET/SKIMMER', desc: 'Activer skimmer' },
    { cmd: 'SET/NOSKIMMER', desc: 'Désactiver skimmer' },
    { cmd: 'SET/FT8', desc: 'Activer FT8' },
    { cmd: 'SET/NOFT8', desc: 'Désactiver FT8' },
    { cmd: 'BYE', desc: 'Déconnexion' },
    { cmd: 'HELP', desc: 'Aide' }
  ];
  
  function sendCommand() {
    if (!command.trim() || wsStatus !== 'connected' || isSending) return;
    
    const cmd = command.trim();
    isSending = true;
    
    // Ajouter à l'historique
    commandHistory.unshift(cmd);
    if (commandHistory.length > 50) commandHistory.pop();
    historyIndex = -1;
    
    // Envoyer via WebSocket
    dispatch('sendCommand', { command: cmd });
    
    // Réinitialiser
    command = '';
    
    // Simuler un délai d'envoi
    setTimeout(() => {
      isSending = false;
    }, 1000);
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
        // Auto-complétion basique
        if (command) {
          const matches = commonCommands.filter(c => 
            c.cmd.toLowerCase().startsWith(command.toLowerCase())
          );
          if (matches.length === 1) {
            command = matches[0].cmd;
          }
        }
        break;
    }
  }
  
  function selectCommonCommand(cmd) {
    command = cmd;
    // Focus sur l'input après un court délai
    setTimeout(() => {
      const input = document.querySelector('#telnet-command-input');
      if (input) input.focus();
    }, 10);
  }
</script>

<div class="h-full flex flex-col overflow-hidden">
  <div class="p-3 border-b border-slate-700/50 flex-shrink-0">
    <h2 class="text-lg font-bold mb-3">Telnet Console</h2>
    
    <div class="flex items-center gap-2 mb-3">
      <div class="flex items-center gap-1.5">
        <div class="w-2 h-2 rounded-full {wsStatus === 'connected' ? 'bg-green-500 animate-pulse' : 'bg-red-500'}"></div>
        <span class="text-xs {wsStatus === 'connected' ? 'text-green-400' : 'text-red-400'}">
          {wsStatus === 'connected' ? 'Connected' : 'Disconnected'}
        </span>
      </div>
      
      {#if isSending}
        <span class="text-xs text-orange-400 animate-pulse">Sending...</span>
      {/if}
    </div>
    
    <!-- Input de commande -->
    <div class="flex gap-2 mb-3">
      <div class="flex-1 relative">
        <input
          id="telnet-command-input"
          type="text"
          bind:value={command}
          on:keydown={handleKeyDown}
          placeholder="Type Telnet command (e.g., SH/DX, SET/FT8...)"
          class="w-full px-3 py-2 bg-slate-700/50 border border-slate-600 rounded text-sm text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 pr-10"
          disabled={wsStatus !== 'connected' || isSending}
        />
        <div class="absolute right-2 top-1/2 transform -translate-y-1/2 text-slate-500 text-xs">
          ↵ Enter
        </div>
      </div>
      <button
        on:click={sendCommand}
        class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-slate-700 disabled:text-slate-500 rounded text-sm font-medium transition-colors flex items-center gap-2"
        disabled={!command.trim() || wsStatus !== 'connected' || isSending}
      >
        {#if isSending}
          <svg class="w-4 h-4 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        {:else}
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
          </svg>
        {/if}
        Send
      </button>
    </div>
    
    <!-- Commandes rapides -->
    <div class="mb-2">
      <h3 class="text-xs text-slate-400 font-semibold mb-1">Quick Commands:</h3>
      <div class="flex flex-wrap gap-1">
        {#each commonCommands as item}
          <button
            on:click={() => selectCommonCommand(item.cmd)}
            class="px-2 py-1 bg-slate-700/50 hover:bg-slate-700 text-slate-300 rounded text-xs transition-colors"
            title={item.desc}
            disabled={wsStatus !== 'connected'}
          >
            {item.cmd}
          </button>
        {/each}
      </div>
    </div>
  </div>
  
  <!-- Historique des commandes -->
  <div class="flex-1 overflow-y-auto p-3">
    <div class="flex items-center justify-between mb-2">
      <h3 class="text-sm font-semibold text-slate-400">Command History</h3>
      {#if commandHistory.length > 0}
        <button
          on:click={() => commandHistory = []}
          class="text-xs text-slate-500 hover:text-slate-300 transition-colors"
        >
          Clear
        </button>
      {/if}
    </div>
    
    {#if commandHistory.length === 0}
      <div class="text-center py-8 text-slate-500">
        <svg class="w-12 h-12 mx-auto mb-3 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
        </svg>
        <p class="text-sm">No commands sent yet</p>
        <p class="text-xs mt-1">Type a command above or select from quick commands</p>
      </div>
    {:else}
      <div class="space-y-1">
        {#each commandHistory as cmd, i}
        {@const commonCmd = commonCommands.find(c => c.cmd === cmd)}
        <div class="flex items-start gap-2 p-2 bg-slate-900/30 rounded hover:bg-slate-800/30 transition-colors">
            <div class="flex-shrink-0 w-6 h-6 flex items-center justify-center bg-slate-700/50 rounded text-xs text-slate-400">
            {commandHistory.length - i}
            </div>
            <div class="flex-1 min-w-0">
            <div class="font-mono text-sm text-blue-300 break-all">{cmd}</div>
            <div class="text-xs text-slate-500 mt-0.5">
                {#if commonCmd}
                {commonCmd.desc}
                {:else}
                Custom command
                {/if}
            </div>
            </div>
            <button
            on:click={() => {
                command = cmd;
                const input = document.querySelector('#telnet-command-input');
                if (input) input.focus();
            }}
            class="px-2 py-1 text-xs bg-slate-700/50 hover:bg-slate-700 text-slate-300 rounded transition-colors"
            title="Re-use this command"
            >
            Reuse
            </button>
        </div>
        {/each}
      </div>
    {/if}
  </div>
</div>