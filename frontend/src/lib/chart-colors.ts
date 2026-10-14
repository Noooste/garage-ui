// Grafana-inspired color palette for charts
export const grafanaColors = {
  light: {
    blue: '#1f77b4',
    orange: '#ff7f0e',
    green: '#2ca02c',
    red: '#d62728',
    purple: '#9467bd',
    brown: '#8c564b',
    pink: '#e377c2',
    gray: '#7f7f7f',
    olive: '#bcbd22',
    cyan: '#17becf',
  },
  dark: {
    blue: '#3eb0ff',
    orange: '#ff9830',
    green: '#73bf69',
    red: '#f2495c',
    purple: '#b581d8',
    brown: '#c49c94',
    pink: '#ff9d96',
    gray: '#9ba3af',
    olive: '#fac858',
    cyan: '#37b7c3',
  },
};

export const chartColorPalette = {
  light: [
    '#1f77b4', // Blue
    '#ff7f0e', // Orange
    '#2ca02c', // Green
    '#d62728', // Red
    '#9467bd', // Purple
    '#8c564b', // Brown
    '#e377c2', // Pink
    '#7f7f7f', // Gray
    '#bcbd22', // Olive
    '#17becf', // Cyan
  ],
  dark: [
    '#3eb0ff', // Bright Blue
    '#ff9830', // Orange
    '#73bf69', // Green
    '#f2495c', // Red
    '#b581d8', // Purple
    '#c49c94', // Brown
    '#ff9d96', // Light Pink
    '#9ba3af', // Gray
    '#fac858', // Yellow
    '#37b7c3', // Cyan
  ],
};

export const getChartColors = (isDark: boolean = false) => {
  return isDark ? chartColorPalette.dark : chartColorPalette.light;
};

// Gray stands for an aggregated "Other" series, so it is left out of the
// colors handed to individual series and never shared with one.
export const getOtherColor = (isDark: boolean = false) => {
  return isDark ? grafanaColors.dark.gray : grafanaColors.light.gray;
};

export const getSeriesColors = (isDark: boolean = false) => {
  const other = getOtherColor(isDark);
  return getChartColors(isDark).filter((color) => color !== other);
};

export const getTextColor = (isDark: boolean = false) => {
  return isDark ? '#e0e0e0' : '#333333';
};

export const getGridColor = (isDark: boolean = false) => {
  return isDark ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.1)';
};

export const getTooltipStyle = (isDark: boolean = false) => {
  return {
    backgroundColor: isDark ? '#1f2937' : '#ffffff',
    border: `1px solid ${isDark ? '#374151' : '#e5e7eb'}`,
    borderRadius: '6px',
    color: isDark ? '#e0e0e0' : '#333333',
  };
};
