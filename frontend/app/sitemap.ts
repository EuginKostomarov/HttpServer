import { MetadataRoute } from 'next'

const baseUrl = process.env.NEXT_PUBLIC_BASE_URL || 'http://localhost:3000'

export default function sitemap(): MetadataRoute.Sitemap {
  return [
    {
      url: baseUrl,
      lastModified: new Date(),
      changeFrequency: 'daily',
      priority: 1,
    },
    {
      url: `${baseUrl}/clients`,
      lastModified: new Date(),
      changeFrequency: 'daily',
      priority: 0.9,
    },
    {
      url: `${baseUrl}/processes`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.9,
    },
    {
      url: `${baseUrl}/quality`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.9,
    },
    {
      url: `${baseUrl}/results`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.8,
    },
    {
      url: `${baseUrl}/databases`,
      lastModified: new Date(),
      changeFrequency: 'daily',
      priority: 0.8,
    },
    {
      url: `${baseUrl}/classifiers`,
      lastModified: new Date(),
      changeFrequency: 'weekly',
      priority: 0.7,
    },
    {
      url: `${baseUrl}/classifiers/kpved`,
      lastModified: new Date(),
      changeFrequency: 'weekly',
      priority: 0.6,
    },
    {
      url: `${baseUrl}/classifiers/okpd2`,
      lastModified: new Date(),
      changeFrequency: 'weekly',
      priority: 0.6,
    },
    {
      url: `${baseUrl}/countries`,
      lastModified: new Date(),
      changeFrequency: 'monthly',
      priority: 0.6,
    },
    {
      url: `${baseUrl}/monitoring`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.7,
    },
    {
      url: `${baseUrl}/workers`,
      lastModified: new Date(),
      changeFrequency: 'daily',
      priority: 0.6,
    },
    {
      url: `${baseUrl}/pipeline-stages`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.6,
    },
    {
      url: `${baseUrl}/models/benchmark`,
      lastModified: new Date(),
      changeFrequency: 'weekly',
      priority: 0.5,
    },
    {
      url: `${baseUrl}/databases/pending`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.7,
    },
    {
      url: `${baseUrl}/monitoring/history`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.6,
    },
    {
      url: `${baseUrl}/quality/duplicates`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.7,
    },
    {
      url: `${baseUrl}/quality/violations`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.7,
    },
    {
      url: `${baseUrl}/quality/suggestions`,
      lastModified: new Date(),
      changeFrequency: 'hourly',
      priority: 0.7,
    },
  ]
}

