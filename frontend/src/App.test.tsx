import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';
import * as productsApi from './api/products';
import type { SearchResult, Product } from './types';

vi.mock('./api/products');
vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  LineChart: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
}));

const mockResults: SearchResult[] = [
  { id: '1', name: 'LECHE ENTERA HACENDADO 1L', category: 'Lácteos', currentPrice: 0.89, minPrice: 0.79, maxPrice: 0.89 },
];

const mockProduct: Product = {
  id: '1',
  name: 'LECHE ENTERA HACENDADO 1L',
  category: 'Lácteos',
  currentPrice: 0.89,
  priceHistory: [
    { date: '2025-01-15T00:00:00Z', price: 0.79, store: 'Mercadona' },
  ],
};

const AUTH_KEY = 'mercaflacion_auth';

function loginAs(username: string, email?: string) {
  localStorage.setItem(
    AUTH_KEY,
    JSON.stringify({ user: { username, email }, token: 'test-token' }),
  );
}

beforeEach(() => {
  localStorage.removeItem(AUTH_KEY);
  vi.mocked(productsApi.getAllProducts).mockResolvedValue([]);
  vi.mocked(productsApi.getAnalytics).mockResolvedValue({
    mostPurchased: [],
    biggestIncreases: [],
  });
  vi.mocked(productsApi.getAccumulatedIPC).mockResolvedValue({
    from_year: 2025,
    to_year: 2025,
    accumulated_rate: 0.025,
  });
});

describe('App', () => {
  it('renders the application logo', () => {
    render(<App />);
    expect(screen.getByRole('img', { name: /mercaflación/i })).toBeInTheDocument();
  });

  it('shows guest screen when not logged in', () => {
    render(<App />);
    expect(screen.getByText(/iniciar sesión/i)).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/buscar producto/i)).not.toBeInTheDocument();
  });

  it('shows SearchBar by default when logged in', () => {
    loginAs('carlos');
    render(<App />);
    expect(screen.getByPlaceholderText(/buscar producto/i)).toBeInTheDocument();
  });

  it('navigates to ProductDetail when a product is selected', async () => {
    loginAs('carlos');
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<App />);
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await userEvent.click(screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await waitFor(() =>
      expect(productsApi.getProduct).toHaveBeenCalledWith('1')
    );
  });

  it('returns to SearchBar when the back button is pressed', async () => {
    loginAs('carlos');
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<App />);
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await userEvent.click(screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await waitFor(() => screen.getByRole('button', { name: /volver/i }));
    await userEvent.click(screen.getByRole('button', { name: /volver/i }));
    expect(screen.getByPlaceholderText(/buscar producto/i)).toBeInTheDocument();
  });

  it('clicking the logo navigates to the home with productos tab and resets to page 1', async () => {
    loginAs('carlos');
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<App />);

    // Navigate to analytics tab
    await userEvent.click(screen.getByRole('tab', { name: /analítica/i }));
    expect(screen.getByRole('tab', { name: /analítica/i })).toHaveAttribute('aria-selected', 'true');

    // Click the logo
    await userEvent.click(screen.getByRole('button', { name: /ir a la página principal/i }));

    // Should return to the products tab
    expect(screen.getByRole('tab', { name: /productos/i })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByPlaceholderText(/buscar producto/i)).toBeInTheDocument();
  });

  it('clicking the logo from ProductDetail returns to home', async () => {
    loginAs('carlos');
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<App />);

    // Navigate to a product
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await userEvent.click(screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await waitFor(() => screen.getByRole('button', { name: /volver/i }));

    // Click the logo
    await userEvent.click(screen.getByRole('button', { name: /ir a la página principal/i }));

    // Should return to the search bar
    expect(screen.getByPlaceholderText(/buscar producto/i)).toBeInTheDocument();
  });
});
