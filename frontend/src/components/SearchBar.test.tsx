import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SearchBar from './SearchBar';
import * as productsApi from '../api/products';
import type { SearchResult } from '../types';

vi.mock('../api/products');
vi.mock('./ProductBrowser', () => ({
  default: ({ onSelectProduct }: { onSelectProduct: (id: string) => void }) => (
    <div data-testid="product-browser">
      <button onClick={() => onSelectProduct('99')}>ProductBrowser</button>
    </div>
  ),
}));

const mockResults: SearchResult[] = [
  { id: '1', name: 'LECHE ENTERA HACENDADO 1L', category: 'Lácteos', currentPrice: 0.89, minPrice: 0.79, maxPrice: 0.89 },
  { id: '8', name: 'YOGUR NATURAL DANONE PACK 4', category: 'Lácteos', currentPrice: 1.79, minPrice: 1.55, maxPrice: 1.79 },
];

beforeEach(() => {
  vi.clearAllMocks();
});

describe('SearchBar', () => {
  it('renders the search input', () => {
    render(<SearchBar onSelectProduct={vi.fn()} />);
    expect(screen.getByPlaceholderText(/buscar producto/i)).toBeInTheDocument();
  });

  it('shows the product browser when nothing has been searched', () => {
    render(<SearchBar onSelectProduct={vi.fn()} />);
    expect(screen.getByTestId('product-browser')).toBeInTheDocument();
  });

  it('hides the product browser once the user starts typing', async () => {
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    render(<SearchBar onSelectProduct={vi.fn()} />);
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => expect(screen.getByText('LECHE ENTERA HACENDADO 1L')).toBeInTheDocument());
    expect(screen.queryByTestId('product-browser')).not.toBeInTheDocument();
  });

  it('forwards onSelectProduct from the product browser', async () => {
    const onSelect = vi.fn();
    render(<SearchBar onSelectProduct={onSelect} />);
    await userEvent.click(screen.getByText('ProductBrowser'));
    expect(onSelect).toHaveBeenCalledWith('99');
  });

  it('shows "Searching..." while loading', async () => {
    vi.mocked(productsApi.searchProducts).mockReturnValue(new Promise(() => {}));
    render(<SearchBar onSelectProduct={vi.fn()} />);
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => expect(screen.getByText('Searching...')).toBeInTheDocument());
  });

  it('shows results after a successful search', async () => {
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    render(<SearchBar onSelectProduct={vi.fn()} />);
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => expect(screen.getByText('LECHE ENTERA HACENDADO 1L')).toBeInTheDocument());
    expect(screen.getByText('YOGUR NATURAL DANONE PACK 4')).toBeInTheDocument();
  });

  it('shows the product category when available', async () => {
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    render(<SearchBar onSelectProduct={vi.fn()} />);
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => expect(screen.getAllByText('Lácteos')).toHaveLength(2));
  });

  it('shows the current price formatted', async () => {
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    render(<SearchBar onSelectProduct={vi.fn()} />);
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => expect(screen.getByText('0,89 €')).toBeInTheDocument());
  });

  it('shows a message when no results are found', async () => {
    vi.mocked(productsApi.searchProducts).mockResolvedValue([]);
    render(<SearchBar onSelectProduct={vi.fn()} />);
    await userEvent.type(screen.getByRole('textbox'), 'xyznonexistent');
    await waitFor(() =>
      expect(screen.getByText(/sin resultados/i)).toBeInTheDocument()
    );
  });

  it('calls onSelectProduct when a result is clicked', async () => {
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    const onSelect = vi.fn();
    render(<SearchBar onSelectProduct={onSelect} />);
    await userEvent.type(screen.getByRole('textbox'), 'leche');
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await userEvent.click(screen.getByText('LECHE ENTERA HACENDADO 1L'));
    expect(onSelect).toHaveBeenCalledWith('1');
  });

  it('clears results when the input is emptied', async () => {
    vi.mocked(productsApi.searchProducts).mockResolvedValue(mockResults);
    render(<SearchBar onSelectProduct={vi.fn()} />);
    const input = screen.getByRole('textbox');
    await userEvent.type(input, 'leche');
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await userEvent.clear(input);
    await waitFor(() =>
      expect(screen.queryByText('LECHE ENTERA HACENDADO 1L')).not.toBeInTheDocument()
    );
  });
});
