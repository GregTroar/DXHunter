<script>
  export let message;
  export let type = 'info';
  
  const icons = {
    success: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>`,
    error: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>`,
    warning: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>`,
    info: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>`,
    milestone: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z"></path>`,
    band: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"></path>`,
    mycall: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z"></path>`,
    radio: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.348 14.651a3.75 3.75 0 010-5.303m5.304 0a3.75 3.75 0 010 5.303m-7.425 2.122a6.75 6.75 0 010-9.546m9.546 0a6.75 6.75 0 010 9.546M5.106 18.894c-3.808-3.808-3.808-9.98 0-13.789m13.788 0c3.808 3.808 3.808 9.981 0 13.79M12 12h.008v.007H12V12zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z"></path>`,
    connection: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>`
  };
  
  const colors = {
    success: 'from-green-500 to-emerald-600',
    error: 'from-red-500 to-rose-600',
    warning: 'from-orange-500 to-amber-600',
    info: 'from-blue-500 to-cyan-600',
    milestone: 'from-purple-500 to-pink-500',
    band: 'from-orange-500 to-amber-500',
    mycall: 'from-red-500 to-pink-500',
    radio: 'from-indigo-500 to-purple-600',
    connection: 'from-green-400 to-emerald-500'
  };
  
  // Animation d'entrée plus dynamique
  let visible = false;
  $: if (message) {
    visible = true;
  }
</script>

<div class="fixed bottom-5 right-5 z-50 animate-in slide-in-from-bottom-5 duration-300" class:opacity-0={!visible}>
  <div class="bg-gradient-to-r {colors[type]} text-white px-5 py-4 rounded-xl shadow-2xl backdrop-blur-sm min-w-[350px] max-w-[500px] border border-white/20">
    <div class="flex items-start gap-3">
      {#if icons[type]}
        <div class="flex-shrink-0 w-8 h-8 bg-white/20 rounded-lg flex items-center justify-center">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            {@html icons[type]}
          </svg>
        </div>
      {/if}
      <div class="flex-1 min-w-0">
        <p class="font-semibold text-sm leading-relaxed break-words">{message}</p>
      </div>
    </div>
  </div>
</div>

<style>
  @keyframes slide-in-from-bottom {
    from {
      transform: translateY(100px);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }
  
  .animate-in {
    animation: slide-in-from-bottom 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  }
</style>