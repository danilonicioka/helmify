import type { NextConfig } from "next";
import fs from 'fs';
import path from 'path';

let chartVersion = '2.0.0';
try {
  chartVersion = fs.readFileSync(path.join(process.cwd(), '../CHART_VERSION'), 'utf-8').trim();
} catch (e) {
  console.warn('Could not read CHART_VERSION, using fallback.', e);
}

const nextConfig: NextConfig = {
  output: 'export',
  env: {
    NEXT_PUBLIC_CHART_VERSION: chartVersion,
  },
};

export default nextConfig;
