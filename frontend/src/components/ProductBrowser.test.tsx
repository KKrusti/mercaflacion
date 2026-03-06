import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ProductBrowser from './ProductBrowser';
import type { ProductBrowserState } from './ProductBrowser';
import * as productsApi from '../api/products';
import type { SearchResult } from '../types';

vi.mock('../api/products');

const mockProducts: SearchResult[] = [
  { id: '1', name: 'LECHE ENTERA HACENDADO 1L', category: 'Lácteos', currentPrice: 0.89, minPrice: 0.79, maxPrice: 0.89 },
  { id: '8', name: 'YOGUR NATURAL DANONE PACK 4', category: 'Lácteos', currentPrice: 1.79, minPrice: 1.55, maxPrice: 1.79 },
  { id: '5', name: 'ARROZ LARGO SOS 1KG', category: 'Arroces y pastas', currentPrice: 1.65, minPrice: 1.39, maxPrice: 1.65 },
  { id: '10', name: 'PLATANOS KG', category: undefined, currentPrice: 1.99, minPrice: 1.69, maxPrice: 1.99 },
];

// 49 products to test pagination (total: 49 > 48 default page size)
const manyProducts: SearchResult[] = Array.from({ length: 49 }, (_, i) => ({
  id: String(i + 1),
  name: `PRODUCTO ${i + 1}`,
  category: 'Test',
  currentPrice: 1.0,
  minPrice: 0.9,
  maxPrice: 1.1,
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('ProductBrowser', () => {
  it('shows skeletons while fetching', () => {
    vi.mocked(productsApi.getAllProducts).mockReturnValue(new Promise(() => {}));
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    expect(screen.getByTestId('browser-skeleton')).toBeInTheDocument();
  });

  it('renders a button for each product', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(mockProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => expect(screen.getByLabelText('LECHE ENTERA HACENDADO 1L')).toBeInTheDocument());
    expect(screen.getByLabelText('YOGUR NATURAL DANONE PACK 4')).toBeInTheDocument();
    expect(screen.getByLabelText('ARROZ LARGO SOS 1KG')).toBeInTheDocument();
  });

  it('renders category labels on each card', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(mockProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));
    // Category labels appear on cards, not as group separators.
    expect(screen.getAllByText('Lácteos').length).toBeGreaterThan(0);
    expect(screen.getByText('Arroces y pastas')).toBeInTheDocument();
  });

  it('shows product price in each card', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(mockProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));
    // Each product shows its price formatted with comma.
    expect(screen.getAllByText(/€/).length).toBeGreaterThan(0);
  });

  it('calls onSelectProduct with the correct id when a button is clicked', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(mockProducts);
    const onSelect = vi.fn();
    render(<ProductBrowser onSelectProduct={onSelect} />);
    await waitFor(() => screen.getByLabelText('LECHE ENTERA HACENDADO 1L'));
    await userEvent.click(screen.getByLabelText('LECHE ENTERA HACENDADO 1L'));
    expect(onSelect).toHaveBeenCalledWith('1');
  });

  it('shows an error message when the API call fails', async () => {
    vi.mocked(productsApi.getAllProducts).mockRejectedValue(new Error('Network error'));
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByText(/no se pudieron cargar los productos/i)).toBeInTheDocument()
    );
  });

  it('switches to 4-column grid when "4 columnas" button is pressed', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(mockProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    const btn4 = screen.getByRole('button', { name: '4 columnas' });
    await userEvent.click(btn4);

    expect(screen.getByTestId('browser-grid').className).toMatch(/browser-grid--4/);
  });

  it('switches back to 3-column grid when "3 columnas" button is pressed', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(mockProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    await userEvent.click(screen.getByRole('button', { name: '4 columnas' }));
    await userEvent.click(screen.getByRole('button', { name: '3 columnas' }));

    expect(screen.getByTestId('browser-grid').className).toMatch(/browser-grid--3/);
  });

  it('shows pagination when products exceed the page size', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(manyProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    // Default page size is 48; 49 products → 2 pages.
    expect(screen.getByRole('navigation', { name: 'Paginación' })).toBeInTheDocument();
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
  });

  it('navigates to the next page', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(manyProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    await userEvent.click(screen.getByRole('button', { name: 'Página siguiente' }));
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
  });

  it('disables previous button on first page', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(manyProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    expect(screen.getByRole('button', { name: 'Página anterior' })).toBeDisabled();
  });

  it('disables next button on last page', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(manyProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    await userEvent.click(screen.getByRole('button', { name: 'Página siguiente' }));
    expect(screen.getByRole('button', { name: 'Página siguiente' })).toBeDisabled();
  });

  it('changing page size resets to page 1', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(manyProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    // Go to page 2.
    await userEvent.click(screen.getByRole('button', { name: 'Página siguiente' }));
    expect(screen.getByText('2 / 2')).toBeInTheDocument();

    // Change page size to 96 (all 49 fit in one page → no pagination rendered).
    await userEvent.click(screen.getByRole('button', { name: '96' }));
    expect(screen.queryByRole('navigation', { name: 'Paginación' })).not.toBeInTheDocument();
  });

  it('does not show pagination when all products fit on one page', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(mockProducts); // only 4 products
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    expect(screen.queryByRole('navigation', { name: 'Paginación' })).not.toBeInTheDocument();
  });

  it('page size options include 12, 24, 48 and 96', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(mockProducts);
    render(<ProductBrowser onSelectProduct={vi.fn()} />);
    await waitFor(() => screen.getByTestId('browser-grid'));

    for (const size of ['12', '24', '48', '96']) {
      expect(screen.getByRole('button', { name: size })).toBeInTheDocument();
    }
  });

  it('restores page when rendered in controlled mode with existing state', async () => {
    vi.mocked(productsApi.getAllProducts).mockResolvedValue(manyProducts);
    const state: ProductBrowserState = { page: 1, pageSize: 48, columns: 3 };
    render(
      <ProductBrowser
        onSelectProduct={vi.fn()}
        browserState={state}
        onBrowserStateChange={vi.fn()}
      />
    );
    await waitFor(() => screen.getByTestId('browser-grid'));

    // Page 2 of 2 should be shown because state says page=1.
    expect(screen.getByText('2 / 2')).toBeInTheDocument();
  });
});
