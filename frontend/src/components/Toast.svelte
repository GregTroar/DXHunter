<script>
  export let message;
  export let type = 'info';

  // Pour les toAll, on extrait le callsign depuis "To ALL de CALL <HHMM> : texte"
  $: parsed = parseMessage(type, message);

  function parseMessage(t, msg) {
    if (t === 'toAll' && msg) {
      const m = msg.match(/^To ALL de ([\w\d\/]+)\s*<(\d{4}Z?)>\s*:\s*(.+)$/i);
      if (m) return { call: m[1], time: m[2], text: m[3].trim() };
      return { call: null, time: null, text: msg };
    }
    return { call: null, time: null, text: msg };
  }

  const pillColors = {
    success:    { bg: 'bg-emerald-900/30', text: 'text-emerald-300', dot: 'bg-emerald-400' },
    error:      { bg: 'bg-red-900/30',     text: 'text-red-300',     dot: 'bg-red-400'     },
    warning:    { bg: 'bg-amber-900/30',   text: 'text-amber-300',   dot: 'bg-amber-400'   },
    info:       { bg: 'bg-slate-700/60',   text: 'text-slate-200',   dot: 'bg-slate-400'   },
    milestone:  { bg: 'bg-purple-900/30',  text: 'text-purple-300',  dot: 'bg-purple-400'  },
    band:       { bg: 'bg-amber-900/30',   text: 'text-amber-300',   dot: 'bg-amber-400'   },
    mycall:     { bg: 'bg-red-900/30',     text: 'text-red-300',     dot: 'bg-red-400'     },
    radio:      { bg: 'bg-indigo-900/30',  text: 'text-indigo-300',  dot: 'bg-indigo-400'  },
    connection: { bg: 'bg-emerald-900/30', text: 'text-emerald-300', dot: 'bg-emerald-400' },
    toAll:      { bg: 'bg-slate-700/60',   text: 'text-sky-300',     dot: 'bg-sky-400'     },
  };

  $: colors = pillColors[type] ?? pillColors.info;
  $: label = type === 'toAll' && parsed.call ? parsed.call : typeLabel(type);

  function typeLabel(t) {
    const map = { success: 'OK', error: 'ERR', warning: 'WARN', info: 'INFO',
                  milestone: 'QSO', band: 'BAND', mycall: 'SPOT', radio: 'RADIO',
                  connection: 'NET', toAll: 'ALL' };
    return map[t] ?? t.toUpperCase();
  }

  let visible = false;
  $: if (message) visible = true;
</script>

<div class="toast-enter">
  <div class="flex items-start gap-2 bg-slate-800/90 border border-slate-700/60
              rounded-2xl px-3 py-2 max-w-[520px]">

    <span class="flex-shrink-0 flex items-center gap-1.5 {colors.bg} {colors.text}
                 text-xs font-medium px-2 py-0.5 rounded-full">
      <span class="w-1.5 h-1.5 rounded-full {colors.dot} flex-shrink-0"></span>
      {label}
    </span>

    {#if type === 'toAll' && parsed.time}
      <span class="flex-shrink-0 text-xs text-slate-500">{parsed.time}</span>
      <span class="flex-shrink-0 text-slate-600 text-xs">·</span>
    {/if}

    <span class="text-xs text-slate-300 leading-relaxed" style="word-break: break-word; overflow-wrap: anywhere;">{type === 'toAll' ? parsed.text : message}</span>

  </div>
</div>

<style>
  .toast-enter {
    animation: toast-slide 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  }
  @keyframes toast-slide {
    from { transform: translateY(12px); opacity: 0; }
    to   { transform: translateY(0);    opacity: 1; }
  }
</style>