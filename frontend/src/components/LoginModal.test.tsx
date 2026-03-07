import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import LoginModal from './LoginModal';

vi.mock('../api/auth', () => ({
  login: vi.fn(),
  register: vi.fn(),
}));

import { login, register } from '../api/auth';
const mockLogin = vi.mocked(login);
const mockRegister = vi.mocked(register);

const mockOnAuth = vi.fn();
const mockOnClose = vi.fn();

function renderModal() {
  return render(<LoginModal onAuth={mockOnAuth} onClose={mockOnClose} />);
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('LoginModal', () => {
  it('renders the login form by default', () => {
    renderModal();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Iniciar sesión')).toBeInTheDocument();
    expect(screen.getByLabelText('Usuario')).toBeInTheDocument();
    expect(screen.getByLabelText('Contraseña')).toBeInTheDocument();
  });

  it('switches to register mode when "Registrarse" is pressed', () => {
    renderModal();
    fireEvent.click(screen.getByRole('button', { name: 'Registrarse' }));
    expect(screen.getByRole('heading', { name: 'Crear cuenta' })).toBeInTheDocument();
  });

  it('calls onClose when the close button is pressed', () => {
    renderModal();
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar' }));
    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it('calls onClose when clicking the overlay', () => {
    renderModal();
    fireEvent.click(screen.getByRole('dialog'));
    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it('calls login with the form credentials', async () => {
    mockLogin.mockResolvedValue({ token: 'tok', user: { userId: 1, username: 'carlos' } });
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Usuario'), { target: { value: 'carlos' } });
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password123' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() => expect(mockLogin).toHaveBeenCalledWith('carlos', 'password123'));
  });

  it('calls onAuth with the token and user after a successful login', async () => {
    mockLogin.mockResolvedValue({ token: 'tok123', user: { userId: 2, username: 'carlos' } });
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Usuario'), { target: { value: 'carlos' } });
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password123' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() =>
      expect(mockOnAuth).toHaveBeenCalledWith({
        token: 'tok123',
        user: { userId: 2, username: 'carlos' },
      }),
    );
  });

  it('shows the error when login fails', async () => {
    mockLogin.mockRejectedValue(new Error('Usuario o contraseña incorrectos'));
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Usuario'), { target: { value: 'carlos' } });
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'wrongpass' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent('Usuario o contraseña incorrectos'),
    );
  });

  it('shows email field in register mode', () => {
    renderModal();
    fireEvent.click(screen.getByRole('button', { name: 'Registrarse' }));
    expect(screen.getByLabelText(/correo electrónico/i)).toBeInTheDocument();
  });

  it('does not show email field in login mode', () => {
    renderModal();
    expect(screen.queryByLabelText(/correo electrónico/i)).not.toBeInTheDocument();
  });

  it('calls register in register mode with username, password and email', async () => {
    mockRegister.mockResolvedValue({ token: 'tok', user: { userId: 3, username: 'nuevo' } });
    const { container } = renderModal();
    fireEvent.click(screen.getByRole('button', { name: 'Registrarse' }));
    fireEvent.change(screen.getByLabelText('Usuario'), { target: { value: 'nuevo' } });
    fireEvent.change(screen.getByLabelText(/correo electrónico/i), { target: { value: 'nuevo@example.com' } });
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password123' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() => expect(mockRegister).toHaveBeenCalledWith('nuevo', 'password123', 'nuevo@example.com'));
  });

  it('disables the button while loading', async () => {
    mockLogin.mockImplementation(() => new Promise(() => {})); // never resolves
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Usuario'), { target: { value: 'carlos' } });
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password123' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() =>
      expect(container.querySelector('button[type="submit"]')).toBeDisabled(),
    );
  });

  it('clears the error when switching tabs', async () => {
    mockLogin.mockRejectedValue(new Error('Error de prueba'));
    const { container } = renderModal();
    fireEvent.change(screen.getByLabelText('Usuario'), { target: { value: 'u' } });
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'p' } });
    fireEvent.click(container.querySelector('button[type="submit"]')!);
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Registrarse' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
