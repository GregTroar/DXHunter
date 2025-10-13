<script>
  export let message;
  export let type = 'info'; // 'success', 'error', 'warning', 'info'
  
  const icons = {
    success: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>`,
    error: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>`,
    warning: `<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>`,
    info: ''
  };
  
  const colors = {
    success: 'bg-green-500',
    error: 'bg-red-500',
    warning: 'bg-orange-500',
    info: 'bg-blue-500'
  };
</script>

<div class="fixed bottom-5 right-5 {colors[type]} text-white px-5 py-3 rounded-lg shadow-lg z-50 animate-in slide-in-from-bottom-5 duration-300">
  <div class="flex items-center gap-2">
    {#if icons[type]}
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        {@html icons[type]}
      </svg>
    {/if}
    <span>{message}</span>
  </div>
</div>

<style>
  @keyframes slide-in-from-bottom {
    from {
      transform: translateY(400px);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }
  
  .animate-in {
    animation: slide-in-from-bottom 0.3s ease-out;
  }
</style>