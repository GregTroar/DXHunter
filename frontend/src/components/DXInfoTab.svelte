<script>
  export let selectedCallsign = '';

  const BANDS = ['160M', '80M', '60M', '40M', '30M', '20M', '17M', '15M', '12M', '10M', '6M'];

  const MODE_COLS = [
    { label: 'Phone', key: 'phone', modes: ['USB', 'LSB', 'SSB', 'AM', 'FM', 'PHONE'] },
    { label: 'CW',    key: 'cw',    modes: ['CW'] },
    { label: 'FT8',   key: 'ft8',   modes: ['FT8'] },
    { label: 'FT4',   key: 'ft4',   modes: ['FT4'] },
    { label: 'RTTY',  key: 'rtty',  modes: ['RTTY', 'PSK', 'PSK31', 'PSK63', 'FSK', 'DIGI'] },
  ];

  let info = null;
  let recentSpots = [];
  let loading = false;
  let error = '';
  let inputCallsign = '';

  // Grid: band -> colKey -> count
  let grid = {};

  function buildGrid(bandModes) {
    const g = {};
    for (const band of BANDS) {
      g[band] = {};
      for (const col of MODE_COLS) {
        g[band][col.key] = 0;
      }
    }
    for (const bm of bandModes || []) {
      const bandUp = bm.band?.toUpperCase();
      const modeUp = bm.mode?.toUpperCase();
      if (!g[bandUp]) continue;
      for (const col of MODE_COLS) {
        if (col.modes.includes(modeUp)) {
          g[bandUp][col.key] += bm.count;
          break;
        }
      }
    }
    return g;
  }

  async function fetchInfo(call) {
    if (!call) return;
    loading = true;
    error = '';
    info = null;
    try {
      const [bmRes, spotsRes] = await Promise.all([
        fetch(`/api/callsign/band-modes?call=${encodeURIComponent(call.toUpperCase())}`),
        fetch(`/api/callsign/spots?call=${encodeURIComponent(call.toUpperCase())}`),
      ]);
      const bmJson = await bmRes.json();
      const spotsJson = await spotsRes.json();
      if (bmJson.success) {
        info = bmJson.data;
        grid = buildGrid(info.bandModes);
      } else {
        error = bmJson.error || 'Error fetching data';
      }
      recentSpots = spotsJson.success ? (spotsJson.data || []) : [];
    } catch (e) {
      error = `Connection error: ${e.message}`;
    } finally {
      loading = false;
    }
  }

  // Auto-fetch when selectedCallsign changes (set from spot click)
  $: if (selectedCallsign && selectedCallsign !== inputCallsign) {
    inputCallsign = selectedCallsign;
    fetchInfo(selectedCallsign);
  }

  function handleSearch() {
    const call = inputCallsign.trim().toUpperCase();
    if (call) fetchInfo(call);
  }

  function handleKey(e) {
    if (e.key === 'Enter') handleSearch();
  }

  function colTotal(colKey) {
    return BANDS.reduce((sum, b) => sum + (grid[b]?.[colKey] ?? 0), 0);
  }

  function formatDate(d) {
    if (!d) return '—';
    return d.slice(0, 10);
  }

  function modeAbbrev(key) {
    switch(key) {
      case 'phone': return 'PH';
      case 'cw':    return 'CW';
      case 'ft8':   return 'FT8';
      case 'ft4':   return 'FT4';
      case 'rtty':  return 'RTTY';
      default:      return key.toUpperCase();
    }
  }

  // Filled square color per mode, matching app theme
  function modeSquareStyle(key) {
    switch(key) {
      case 'phone': return 'background:#3b82f6;';          // blue-500
      case 'cw':    return 'background:#f97316;';          // orange-500
      case 'ft8':   return 'background:#a855f7;';          // purple-500
      case 'ft4':   return 'background:#ec4899;';          // pink-500
      case 'rtty':  return 'background:#eab308;';          // yellow-500
      default:      return 'background:#6b7280;';
    }
  }

  // Country name → ISO 3166-1 alpha-2 code (used for flag image URL)
  function countryISO(countryName) {
    if (!countryName) return '';
    const isoMap = {
      // Europe
      'Albania': 'AL', 'Andorra': 'AD', 'Austria': 'AT', 'Azerbaijan': 'AZ',
      'Belarus': 'BY', 'Belgium': 'BE', 'Bosnia-Herzegovina': 'BA', 'Bulgaria': 'BG',
      'Croatia': 'HR', 'Cyprus': 'CY', 'Czech Republic': 'CZ', 'Czechia': 'CZ',
      'Denmark': 'DK', 'Estonia': 'EE', 'Finland': 'FI', 'France': 'FR',
      'Germany': 'DE', 'Gibraltar': 'GI', 'Greece': 'GR', 'Hungary': 'HU',
      'Iceland': 'IS', 'Ireland': 'IE', 'Italy': 'IT', 'Kosovo': 'XK',
      'Latvia': 'LV', 'Liechtenstein': 'LI', 'Lithuania': 'LT', 'Luxembourg': 'LU',
      'Malta': 'MT', 'Moldova': 'MD', 'Monaco': 'MC', 'Montenegro': 'ME',
      'Netherlands': 'NL', 'North Macedonia': 'MK', 'Norway': 'NO', 'Poland': 'PL',
      'Portugal': 'PT', 'Romania': 'RO', 'San Marino': 'SM', 'Serbia': 'RS',
      'Slovakia': 'SK', 'Slovenia': 'SI', 'Spain': 'ES', 'Sweden': 'SE',
      'Switzerland': 'CH', 'Turkey': 'TR', 'Ukraine': 'UA', 'United Kingdom': 'GB',
      'Vatican City': 'VA', 'Armenia': 'AM', 'Georgia': 'GE',
      // Russia / regional
      'European Russia': 'RU', 'Asiatic Russia': 'RU', 'Russia': 'RU', 'Kaliningrad': 'RU',
      // UK territories / islands
      'Channel Islands': 'GG', 'Isle of Man': 'IM',
      'Azores': 'PT', 'Madeira': 'PT', 'Canary Islands': 'ES', 'Balearic Islands': 'ES',
      // North America
      'Canada': 'CA', 'Mexico': 'MX',
      'Alaska': 'US', 'Hawaii': 'US', 'USA': 'US', 'United States': 'US',
      'Puerto Rico': 'PR', 'Guam': 'GU', 'US Virgin Islands': 'VI',
      'Northern Mariana Islands': 'MP',
      // Central America & Caribbean
      'Belize': 'BZ', 'Costa Rica': 'CR', 'Cuba': 'CU', 'Dominican Republic': 'DO',
      'El Salvador': 'SV', 'Guatemala': 'GT', 'Haiti': 'HT', 'Honduras': 'HN',
      'Jamaica': 'JM', 'Nicaragua': 'NI', 'Panama': 'PA', 'Trinidad and Tobago': 'TT',
      'Barbados': 'BB', 'Martinique': 'MQ', 'Guadeloupe': 'GP', 'Aruba': 'AW',
      'Curacao': 'CW', 'Sint Maarten': 'SX', 'Bahamas': 'BS', 'Cayman Islands': 'KY',
      'Bermuda': 'BM', 'Grenada': 'GD', 'Saint Lucia': 'LC', 'Dominica': 'DM',
      'Antigua and Barbuda': 'AG', 'Montserrat': 'MS',
      // South America
      'Argentina': 'AR', 'Bolivia': 'BO', 'Brazil': 'BR', 'Chile': 'CL',
      'Colombia': 'CO', 'Ecuador': 'EC', 'Falkland Islands': 'FK', 'French Guiana': 'GF',
      'Guyana': 'GY', 'Paraguay': 'PY', 'Peru': 'PE', 'Suriname': 'SR',
      'Uruguay': 'UY', 'Venezuela': 'VE',
      // Africa
      'Algeria': 'DZ', 'Angola': 'AO', 'Benin': 'BJ', 'Botswana': 'BW',
      'Burkina Faso': 'BF', 'Cameroon': 'CM', 'Cape Verde': 'CV',
      'Central Africa': 'CF', 'Chad': 'TD', 'Comoros': 'KM', 'Congo': 'CG',
      'DR Congo': 'CD', 'Egypt': 'EG', 'Ethiopia': 'ET', 'Gabon': 'GA',
      'Ghana': 'GH', 'Guinea': 'GN', 'Ivory Coast': 'CI', 'Kenya': 'KE',
      'Libya': 'LY', 'Madagascar': 'MG', 'Malawi': 'MW', 'Mali': 'ML',
      'Mauritania': 'MR', 'Mauritius': 'MU', 'Morocco': 'MA', 'Mozambique': 'MZ',
      'Namibia': 'NA', 'Niger': 'NE', 'Nigeria': 'NG', 'Reunion': 'RE',
      'Rwanda': 'RW', 'Senegal': 'SN', 'Seychelles': 'SC', 'Sierra Leone': 'SL',
      'Somalia': 'SO', 'South Africa': 'ZA', 'Sudan': 'SD', 'Tanzania': 'TZ',
      'Togo': 'TG', 'Tunisia': 'TN', 'Uganda': 'UG', 'Zambia': 'ZM', 'Zimbabwe': 'ZW',
      'Western Sahara': 'EH',
      // Middle East
      'Bahrain': 'BH', 'Iran': 'IR', 'Iraq': 'IQ', 'Israel': 'IL',
      'Jordan': 'JO', 'Kuwait': 'KW', 'Lebanon': 'LB', 'Oman': 'OM',
      'Qatar': 'QA', 'Saudi Arabia': 'SA', 'Syria': 'SY',
      'United Arab Emirates': 'AE', 'Yemen': 'YE', 'Palestine': 'PS',
      // Asia
      'Afghanistan': 'AF', 'Bangladesh': 'BD', 'Bhutan': 'BT', 'Brunei': 'BN',
      'Cambodia': 'KH', 'China': 'CN', 'Hong Kong': 'HK', 'Macao': 'MO',
      'India': 'IN', 'Indonesia': 'ID', 'Japan': 'JP', 'Kazakhstan': 'KZ',
      'Kyrgyzstan': 'KG', 'Laos': 'LA', 'Maldives': 'MV', 'Malaysia': 'MY',
      'Mongolia': 'MN', 'Myanmar': 'MM', 'Nepal': 'NP', 'North Korea': 'KP',
      'Pakistan': 'PK', 'Philippines': 'PH', 'Singapore': 'SG', 'South Korea': 'KR',
      'Sri Lanka': 'LK', 'Taiwan': 'TW', 'Tajikistan': 'TJ', 'Thailand': 'TH',
      'Turkmenistan': 'TM', 'Uzbekistan': 'UZ', 'Vietnam': 'VN', 'Korea': 'KR',
      // Pacific & Oceania
      'Australia': 'AU', 'Fiji': 'FJ', 'French Polynesia': 'PF',
      'Kiribati': 'KI', 'Marshall Islands': 'MH', 'Micronesia': 'FM',
      'Nauru': 'NR', 'New Caledonia': 'NC', 'New Zealand': 'NZ',
      'Palau': 'PW', 'Papua New Guinea': 'PG', 'Samoa': 'WS', 'Solomon Islands': 'SB',
      'Tonga': 'TO', 'Tuvalu': 'TV', 'Vanuatu': 'VU', 'Wallis and Futuna': 'WF',
      'Cook Islands': 'CK', 'Norfolk Island': 'NF', 'Cocos Islands': 'CC',
      'Christmas Island': 'CX',
    };
    return isoMap[countryName] ?? '';
  }
</script>

<div class="flex flex-col h-full p-3 gap-3 overflow-y-auto">

  <!-- Search bar -->
  <div class="flex gap-2">
    <input
      type="text"
      bind:value={inputCallsign}
      on:keydown={handleKey}
      placeholder="Enter callsign (e.g. T31TTT)"
      class="flex-1 bg-slate-800 border border-slate-600 rounded px-3 py-1.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500"
    />
    <button
      on:click={handleSearch}
      class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded font-semibold transition-colors"
    >
      Search
    </button>
  </div>

  {#if loading}
    <div class="text-slate-400 text-sm text-center py-4">Loading…</div>

  {:else if error}
    <div class="text-red-400 text-sm text-center py-4">{error}</div>

  {:else if info}
    <!-- Header info -->
    <div class="bg-slate-800/60 rounded-lg border border-slate-700/50 p-3 flex flex-wrap gap-4 items-center">
      <div class="flex items-center gap-2">
        {#if countryISO(info.country)}
          <img
            src="https://flagcdn.com/20x15/{countryISO(info.country).toLowerCase()}.png"
            alt={info.country}
            style="width:24px; height:18px; object-fit:cover; border-radius:2px;"
          />
        {/if}
        <div>
          <span class="text-xl font-bold text-white">{info.callsign}</span>
          {#if info.country}
            <span class="ml-2 text-slate-400 text-sm">{info.country}</span>
          {/if}
          {#if info.dxcc}
            <span class="ml-1 text-xs text-slate-500">({info.dxcc})</span>
          {/if}
        </div>
      </div>
      <div class="flex gap-4 text-sm ml-auto">
        <div class="text-center">
          <div class="text-blue-400 font-bold text-lg">{info.totalQSOs}</div>
          <div class="text-slate-500 text-xs">Total QSOs</div>
        </div>
        {#if info.firstQSO}
          <div class="text-center">
            <div class="text-slate-300 font-semibold">{formatDate(info.firstQSO)}</div>
            <div class="text-slate-500 text-xs">First QSO</div>
          </div>
        {/if}
        {#if info.lastQSO}
          <div class="text-center">
            <div class="text-slate-300 font-semibold">{formatDate(info.lastQSO)}</div>
            <div class="text-slate-500 text-xs">Last QSO</div>
          </div>
        {/if}
      </div>
    </div>

    <!-- Band/Mode grid + Recent spots side by side -->
    <div class="flex gap-3 items-start">

      <!-- Band/Mode compact grid: bands on top, modes on left, small colored squares -->
      <div class="bg-slate-800/60 rounded-lg border border-slate-700/50 p-2.5 flex-none">
        <div class="flex flex-col gap-0.5">
          <!-- Band column headers -->
          <div class="flex items-end mb-0.5">
            <div class="flex-none" style="width:40px;"></div>
            {#each BANDS as band}
              <div class="flex-none text-center" style="width:20px; font-size:9px; color:#64748b; font-weight:600; line-height:1.4; letter-spacing:-0.5px;">
                {band.replace('M','')}
              </div>
            {/each}
          </div>
          <!-- One row per mode -->
          {#each MODE_COLS as col}
            <div class="flex items-center">
              <div class="flex-none text-right pr-1.5" style="width:40px; font-size:10px; color:#94a3b8; font-weight:700; letter-spacing:0.3px;">
                {modeAbbrev(col.key)}
              </div>
              {#each BANDS as band}
                {@const count = grid[band]?.[col.key] ?? 0}
                <div
                  class="flex-none rounded-sm"
                  title="{band} {col.label}: {count} QSO{count !== 1 ? 's' : ''}"
                  style="width:17px; height:17px; margin:1.5px; {count > 0 ? modeSquareStyle(col.key) : 'background:rgba(15,23,42,0.9); border:1px solid rgba(51,65,85,0.5);'}"
                ></div>
              {/each}
            </div>
          {/each}
        </div>
      </div>

      <!-- Recent spots -->
      {#if recentSpots.length > 0}
        <div class="bg-slate-800/60 rounded-lg border border-slate-700/50 overflow-hidden flex-1 min-w-0">
          <div class="px-3 py-2 border-b border-slate-700/50 bg-slate-900/40">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wide">Last {recentSpots.length} spots</span>
          </div>
          <table class="w-full text-xs">
            <thead>
              <tr class="border-b border-slate-700/30 text-slate-500">
                <th class="py-1.5 px-2 text-left">Time</th>
                <th class="py-1.5 px-2 text-left">Freq</th>
                <th class="py-1.5 px-2 text-left">Band</th>
                <th class="py-1.5 px-2 text-left">Mode</th>
                <th class="py-1.5 px-2 text-left">Spotter</th>
                <th class="py-1.5 px-2 text-left">Comment</th>
              </tr>
            </thead>
            <tbody>
              {#each recentSpots as s}
                <tr class="border-b border-slate-700/20 hover:bg-slate-700/20">
                  <td class="py-1.5 px-2 text-slate-400 font-mono">{s.UTCTime}</td>
                  <td class="py-1.5 px-2 font-mono text-slate-200">{parseFloat(s.FrequencyMhz).toFixed(3)}</td>
                  <td class="py-1.5 px-2">
                    <span class="px-1.5 py-0.5 bg-slate-700/50 rounded text-slate-300">{s.Band}</span>
                  </td>
                  <td class="py-1.5 px-2">
                    <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded">{s.Mode}</span>
                  </td>
                  <td class="py-1.5 px-2 text-slate-400">{s.SpotterCallsign}</td>
                  <td class="py-1.5 px-2 text-slate-500 truncate max-w-[160px]" title={s.OriginalComment}>{s.OriginalComment || '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

    </div>

  {:else}
    <div class="flex flex-col items-center justify-center py-12 text-slate-500 gap-2">
      <svg class="w-10 h-10 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
          d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
      </svg>
      <p class="text-sm">Click a spot or enter a callsign to see band/mode stats</p>
    </div>
  {/if}
</div>
