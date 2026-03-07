import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import HouseholdSection from './HouseholdSection';
import * as householdApi from '../api/household';

vi.mock('../api/household');

const mockGetHousehold = vi.mocked(householdApi.getHousehold);
const mockCreateInvitation = vi.mocked(householdApi.createInvitation);
const mockLeaveHousehold = vi.mocked(householdApi.leaveHousehold);

const TOKEN = 'test-token';
const CURRENT_USER = 'alice';

function renderSection(onLeft = vi.fn()) {
  return render(
    <HouseholdSection token={TOKEN} currentUsername={CURRENT_USER} onLeft={onLeft} />,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('HouseholdSection', () => {
  it('shows empty state when user has no household', async () => {
    mockGetHousehold.mockResolvedValue([]);
    renderSection();
    await waitFor(() => {
      expect(mockGetHousehold).toHaveBeenCalledWith(TOKEN);
    });
    expect(screen.queryByRole('list')).toBeNull();
    expect(screen.getByRole('button', { name: /invitar conviviente/i })).toBeInTheDocument();
  });

  it('renders members list with (tú) marker for current user', async () => {
    mockGetHousehold.mockResolvedValue([
      { id: 1, username: 'alice' },
      { id: 2, username: 'bob' },
    ]);
    renderSection();
    await waitFor(() => {
      expect(screen.getByText('alice')).toBeInTheDocument();
    });
    expect(screen.getByText(/\(tú\)/)).toBeInTheDocument();
    expect(screen.getByText('bob')).toBeInTheDocument();
  });

  it('shows invite link after clicking Invitar conviviente', async () => {
    mockGetHousehold.mockResolvedValue([]);
    mockCreateInvitation.mockResolvedValue('tok123');
    renderSection();
    await waitFor(() => expect(mockGetHousehold).toHaveBeenCalled());

    await userEvent.click(screen.getByRole('button', { name: /invitar conviviente/i }));

    await waitFor(() => {
      expect(screen.getByRole('textbox', { name: /enlace de invitación/i })).toBeInTheDocument();
    });
    const input = screen.getByRole('textbox', { name: /enlace de invitación/i }) as HTMLInputElement;
    expect(input.value).toMatch(/invite=tok123/);
    expect(screen.getByText(/válido durante 24 horas/i)).toBeInTheDocument();
  });

  it('shows error message when createInvitation fails', async () => {
    mockGetHousehold.mockResolvedValue([]);
    mockCreateInvitation.mockRejectedValue(new Error('network error'));
    renderSection();
    await waitFor(() => expect(mockGetHousehold).toHaveBeenCalled());

    await userEvent.click(screen.getByRole('button', { name: /invitar conviviente/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/no se pudo crear la invitación/i);
    });
  });

  it('calls onLeft after leaving household', async () => {
    mockGetHousehold.mockResolvedValue([{ id: 1, username: 'alice' }]);
    mockLeaveHousehold.mockResolvedValue(undefined);
    const onLeft = vi.fn();
    renderSection(onLeft);
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: /abandonar unidad familiar/i }));

    await waitFor(() => expect(onLeft).toHaveBeenCalledTimes(1));
  });

  it('shows error when leaveHousehold fails', async () => {
    mockGetHousehold.mockResolvedValue([{ id: 1, username: 'alice' }]);
    mockLeaveHousehold.mockRejectedValue(new Error('network error'));
    renderSection();
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: /abandonar unidad familiar/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/no se pudo abandonar/i);
    });
  });

  it('copies invite link to clipboard', async () => {
    mockGetHousehold.mockResolvedValue([]);
    mockCreateInvitation.mockResolvedValue('tok456');
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      writable: true,
      configurable: true,
    });
    renderSection();
    await waitFor(() => expect(mockGetHousehold).toHaveBeenCalled());

    await userEvent.click(screen.getByRole('button', { name: /invitar conviviente/i }));
    await waitFor(() => screen.getByRole('button', { name: /copiar enlace/i }));

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /copiar enlace/i }));
    });

    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('tok456'));
  });
});
