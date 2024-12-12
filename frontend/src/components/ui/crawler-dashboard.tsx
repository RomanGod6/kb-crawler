'use client';

import React, { useState, useEffect } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Download, Plus, X, Pencil } from 'lucide-react';
import Papa from 'papaparse';

interface CrawlerEntry {
    id: number;
    product: string;
    sitemapUrl: string;
    mapUrl?: string;
    userAgent?: string;
    crawlInterval?: string;
    maxDepth?: number;
    defaultCategory?: string;
    allowedDomains?: string[];
    status: 'Running' | 'Stopped' | 'Error';
    dateAdded: string;
    dateModified: string;
    lastRunTime: string | null;
    logs: string[];
}

const initialFormData: Partial<CrawlerEntry> = {
    product: '',
    sitemapUrl: '',
    mapUrl: '',
    userAgent: 'KIT Crawler', // Default User-Agent
    crawlInterval: '24h', // Default Crawl Interval
    maxDepth: 15, // Default Max Depth
    defaultCategory: '',
    allowedDomains: [],
    status: 'Stopped',
    dateAdded: '',
    dateModified: '',
    lastRunTime: null,
    logs: [],
};

const CrawlerDashboard: React.FC = () => {
    const [entries, setEntries] = useState<CrawlerEntry[]>([]);
    const [showForm, setShowForm] = useState(false);
    const [formData, setFormData] = useState<Partial<CrawlerEntry>>(initialFormData);
    const [editingId, setEditingId] = useState<number | null>(null);

    useEffect(() => {
        fetchEntries();
    }, []);

    const fetchEntries = async () => {
        try {
            const response = await fetch('http://localhost:8080/api/crawlers');
            const data = await response.json();

            if (!data || data.length === 0) {
                console.log('No crawlers available');
                setEntries([]);
            } else {
                setEntries(data);
            }
        } catch (error) {
            console.error('Error fetching crawlers:', error);
        }
    };

    const handleChange = (
        e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>
    ) => {
        const { name, value } = e.target;
        setFormData((prev) => ({
            ...prev,
            [name]: name === 'maxDepth' ? parseInt(value, 10) || 0 : value,
        }));
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
    
        try {
            const payload = {
                ...formData,
                allowedDomains: Array.isArray(formData.allowedDomains)
                    ? formData.allowedDomains
                    : formData.allowedDomains?.split(',').map((domain: string) => domain.trim()) || [],
                dateModified: new Date().toISOString(),
                dateAdded: formData.dateAdded || new Date().toISOString(),
                status: 'Running',
            };
    
            if (editingId !== null) {
                // Update existing entry
                await fetch(`http://localhost:8080/api/crawlers/${editingId}`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(payload),
                });
            } else {
                // Create new entry
                await fetch('http://localhost:8080/api/crawlers', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(payload),
                });
            }
    
            fetchEntries(); // Refresh the entries after successful submission
            setFormData(initialFormData);
            setEditingId(null);
            setShowForm(false);
        } catch (error) {
            console.error('Error saving entry:', error);
        }
    };
    const handleEdit = (entry: CrawlerEntry) => {
        setFormData(entry);
        setEditingId(entry.id);
        setShowForm(true);
    };
    
    const getStatusBadgeClasses = (status: string) => {
        if (status === 'Running') return 'bg-green-100 text-green-800';
        if (status === 'Error') return 'bg-red-100 text-red-800';
        return 'bg-gray-100 text-gray-800';
    };
    const handleCancel = () => {
        setFormData(initialFormData);
        setEditingId(null);
        setShowForm(false);
    };

    const exportToCSV = () => {
        const csv = Papa.unparse(entries);
        const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
        const link = document.createElement('a');
        link.href = URL.createObjectURL(blob);
        link.download = `crawler_entries_${new Date().toISOString().split('T')[0]}.csv`;
        link.click();
    };

    return (
        <div className="p-4 max-w-6xl mx-auto">
            <div className="flex justify-between items-center mb-6">
                <h1 className="text-2xl font-bold">KIT Web Crawler</h1>
                <div className="space-x-4">
                    <button
                        onClick={() => {
                            setFormData(initialFormData);
                            setEditingId(null);
                            setShowForm(true);
                        }}
                        className="bg-blue-500 text-white px-4 py-2 rounded-md hover:bg-blue-600 inline-flex items-center"
                    >
                        <Plus className="w-4 h-4 mr-2" /> Add Entry
                    </button>
                    <button
                        onClick={exportToCSV}
                        className="bg-green-500 text-white px-4 py-2 rounded-md hover:bg-green-600 inline-flex items-center"
                    >
                        <Download className="w-4 h-4 mr-2" /> Export CSV
                    </button>
                </div>
            </div>

            {showForm && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center">
                          <Card className="w-full max-w-2xl max-h-[90vh] overflow-y-auto">
                    <Card className="w-full max-w-2xl">
                        <CardHeader className="flex justify-between">
                            <CardTitle>{editingId ? 'Edit Entry' : 'Add Entry'}</CardTitle>
                            <button onClick={handleCancel} className="p-2">
                                <X className="w-4 h-4" />
                            </button>
                        </CardHeader>
                        <CardContent>
                            <form onSubmit={handleSubmit} className="space-y-4">
                                <div>
                                    <label>Product</label>
                                    <input
                                        type="text"
                                        name="product"
                                        value={formData.product || ''}
                                        onChange={handleChange}
                                        className="w-full p-2 border rounded-md"
                                        required
                                    />
                                </div>
                                <div>
                                    <label>Sitemap URL</label>
                                    <input
                                        type="url"
                                        name="sitemapUrl"
                                        value={formData.sitemapUrl || ''}
                                        onChange={handleChange}
                                        className="w-full p-2 border rounded-md"
                                        required
                                    />
                                </div>
                                <div>
                                    <label>Map URL</label>
                                    <input
                                        type="url"
                                        name="mapUrl"
                                        value={formData.mapUrl || ''}
                                        onChange={handleChange}
                                        className="w-full p-2 border rounded-md"
                                    />
                                </div>
                                <div>
                                    <label>User Agent</label>
                                    <input
                                        type="text"
                                        name="userAgent"
                                        value={formData.userAgent || 'MyCrawler/1.0'}
                                        onChange={handleChange}
                                        className="w-full p-2 border rounded-md"
                                    />
                                </div>
                                <div>
                                    <label>Crawl Interval</label>
                                    <input
                                        type="text"
                                        name="crawlInterval"
                                        value={formData.crawlInterval || '24h'}
                                        onChange={handleChange}
                                        className="w-full p-2 border rounded-md"
                                    />
                                </div>
                                <div>
                                    <label>Max Depth</label>
                                    <input
                                        type="number"
                                        name="maxDepth"
                                        value={formData.maxDepth || 3}
                                        onChange={handleChange}
                                        className="w-full p-2 border rounded-md"
                                    />
                                </div>
                                <div>
                                    <label>Default Category</label>
                                    <input
                                        type="text"
                                        name="defaultCategory"
                                        value={formData.defaultCategory || ''}
                                        onChange={handleChange}
                                        className="w-full p-2 border rounded-md"
                                    />
                                </div>
                                <div>
                                    <label>Allowed Domains (comma-separated)</label>
                                    <textarea
    name="allowedDomains"
    value={formData.allowedDomains?.join(', ') || ''} 
    onChange={(e) =>
        setFormData((prev) => ({
            ...prev,
            allowedDomains: e.target.value.split(',').map((domain) => domain.trim()), 
        }))
    }
    className="w-full p-2 border rounded-md h-24"
/>
                                </div>
                                <div className="flex justify-end space-x-4">
                                    <button
                                        type="button"
                                        onClick={handleCancel}
                                        className="bg-gray-300 px-4 py-2 rounded-md"
                                    >
                                        Cancel
                                    </button>
                                    <button
                                        type="submit"
                                        className="bg-blue-500 text-white px-4 py-2 rounded-md"
                                    >
                                        {editingId ? 'Update' : 'Add'}
                                    </button>
                                </div>
                            </form>
                        </CardContent>
                    </Card>
                    </Card>
                </div>
            )}

            <Card>
                <CardHeader>
                    <CardTitle>Entries</CardTitle>
                </CardHeader>
                <CardContent>
                    <table className="w-full table-auto">
                        <thead>
                            <tr>
                                <th>Product</th>
                                <th>Sitemap URL</th>
                                <th>Status</th>
                                <th>Date Added</th>
                                <th>Date Modified</th>
                                <th>Last Run</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {entries.map((entry) => (
                                <tr key={entry.id}>
                                    <td>{entry.product}</td>
                                    <td>
                                        <a
                                            href={entry.sitemapUrl}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            className="text-blue-500 underline"
                                        >
                                            {entry.sitemapUrl}
                                        </a>
                                    </td>
                                    <td>
                                        <span className={`px-2 py-1 rounded ${getStatusBadgeClasses(entry.status)}`}>
                                            {entry.status}
                                        </span>
                                    </td>
                                    <td>{new Date(entry.dateAdded).toLocaleDateString()}</td>
                                    <td>{new Date(entry.dateModified).toLocaleDateString()}</td>
                                    <td>{entry.lastRunTime || 'Never'}</td>
                                    <td>
                                        <button
                                            onClick={() => handleEdit(entry)}
                                            className="text-blue-500"
                                        >
                                            <Pencil className="w-4 h-4" />
                                        </button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </CardContent>
            </Card>
        </div>
    );
};

export default CrawlerDashboard;
