import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Временно отключаем standalone для устранения проблем с prerendering
  // output: 'standalone',
  
  // Настройки для работы в контейнере
  // outputFileTracingRoot: require('path').join(__dirname, '../../'),
  
  // Включаем строгий режим React для лучшей работы с DevTools
  reactStrictMode: true,
};

export default nextConfig;
