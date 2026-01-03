import React, { useEffect, useState, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import {
  CSDLayoutPage,
  CSDPaper,
  CSDTypography,
  CSDStack,
  CSDBox,
  CSDIcon,
  CSDButton,
  CSDChip,
  CSDTerminalDialog,
} from 'csd_core/UI';
import { useTranslation } from '../../../translations/TranslationContext';
import { useGraphQL } from '../../../shared/hooks';
import { formatDateTime, formatRelativeTime } from '../../../shared/utils';
import type { Session, Message } from '../../../types';

// Terminal token response from backend
interface TerminalTokenResponse {
  token: string;      // Opaque token (frontend never sees session name or prefix)
  expiresIn: number;  // Expiration in seconds
}

const SessionDetailPage: React.FC = () => {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { request } = useGraphQL();
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [terminalOpen, setTerminalOpen] = useState(searchParams.get('terminal') === 'true');
  const [terminalToken, setTerminalToken] = useState<string | null>(null);

  useEffect(() => {
    const loadData = async () => {
      if (!id) return;
      try {
        setLoading(true);
        const result = await request<{ session: Session }>(`
          query($id: String!) {
            session(id: $id) {
              id name projectId projectName workDir state
              messages { id role content timestamp }
              createdAt lastActiveAt error isRealSession sessionFile
            }
          }
        `, { id });
        setSession(result.session);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load session');
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, [id, request]);

  const getStateColor = (state: string): 'success' | 'warning' | 'error' | 'default' => {
    switch (state) {
      case 'running': return 'success';
      case 'waiting': return 'warning';
      case 'error': return 'error';
      default: return 'default';
    }
  };

  // Create terminal token from backend
  // The token is opaque - the frontend never sees the session name or prefix
  const createTerminalToken = useCallback(async () => {
    if (!session?.id) return null;
    try {
      const result = await request<{ createTerminalToken: TerminalTokenResponse }>(`
        mutation($sessionId: String!, $sessionType: String) {
          createTerminalToken(sessionId: $sessionId, sessionType: $sessionType) {
            token
            expiresIn
          }
        }
      `, { sessionId: session.id, sessionType: 'claude' });
      return result.createTerminalToken.token;
    } catch (err) {
      console.error('Failed to create terminal token:', err);
      return null;
    }
  }, [session?.id, request]);

  const handleOpenTerminal = async () => {
    // Get terminal token from backend
    // The session name/prefix will be resolved server-side when csd-core validates the token
    const token = await createTerminalToken();
    setTerminalToken(token);
    setTerminalOpen(true);
  };

  const handleCloseTerminal = () => {
    setTerminalOpen(false);
    setTerminalToken(null);
    // Remove terminal param from URL
    navigate(`/devtrack/sessions/${id}`, { replace: true });
  };

  const handleResumeSession = async () => {
    if (!session) return;
    // Resume session via Claude CLI - this would open a terminal with the session
    await handleOpenTerminal();
  };

  if (loading) {
    return (
      <CSDLayoutPage title={t('common.loading')}>
        <CSDBox sx={{ p: 3, textAlign: 'center' }}>
          <CSDTypography>{t('common.loading')}</CSDTypography>
        </CSDBox>
      </CSDLayoutPage>
    );
  }

  if (error || !session) {
    return (
      <CSDLayoutPage title={t('common.error')}>
        <CSDBox sx={{ p: 3, textAlign: 'center' }}>
          <CSDTypography color="error">{error || 'Session not found'}</CSDTypography>
          <CSDButton onClick={() => navigate('/devtrack/sessions')} sx={{ mt: 2 }}>
            {t('common.back')}
          </CSDButton>
        </CSDBox>
      </CSDLayoutPage>
    );
  }

  return (
    <CSDLayoutPage
      title={session.name}
      breadcrumbs={[
        { label: t('sessions.title'), path: '/devtrack/sessions' },
        { label: session.name },
      ]}
      actions={
        <CSDStack direction="row" spacing={1}>
          <CSDButton
            variant="outlined"
            startIcon={<CSDIcon>terminal</CSDIcon>}
            onClick={handleOpenTerminal}
          >
            {t('sessions.openTerminal')}
          </CSDButton>
          <CSDButton
            variant="contained"
            startIcon={<CSDIcon>play_arrow</CSDIcon>}
            onClick={handleResumeSession}
          >
            {t('sessions.resume')}
          </CSDButton>
        </CSDStack>
      }
    >
      <CSDStack spacing={3}>
        {/* Session Info */}
        <CSDPaper sx={{ p: 3 }}>
          <CSDStack spacing={2}>
            <CSDStack direction="row" spacing={2} alignItems="center">
              <CSDTypography variant="body2" color="text.secondary" sx={{ width: 120 }}>
                State:
              </CSDTypography>
              <CSDChip
                label={t(`sessions.${session.state}`)}
                color={getStateColor(session.state)}
                size="small"
              />
            </CSDStack>
            <CSDStack direction="row" spacing={2} alignItems="center">
              <CSDTypography variant="body2" color="text.secondary" sx={{ width: 120 }}>
                Project:
              </CSDTypography>
              <CSDTypography>{session.projectName || '-'}</CSDTypography>
            </CSDStack>
            <CSDStack direction="row" spacing={2} alignItems="center">
              <CSDTypography variant="body2" color="text.secondary" sx={{ width: 120 }}>
                Working Dir:
              </CSDTypography>
              <CSDTypography fontFamily="monospace" variant="body2">
                {session.workDir}
              </CSDTypography>
            </CSDStack>
            <CSDStack direction="row" spacing={2} alignItems="center">
              <CSDTypography variant="body2" color="text.secondary" sx={{ width: 120 }}>
                Created:
              </CSDTypography>
              <CSDTypography>{formatDateTime(session.createdAt)}</CSDTypography>
            </CSDStack>
            <CSDStack direction="row" spacing={2} alignItems="center">
              <CSDTypography variant="body2" color="text.secondary" sx={{ width: 120 }}>
                Last Active:
              </CSDTypography>
              <CSDTypography>{formatRelativeTime(session.lastActiveAt)}</CSDTypography>
            </CSDStack>
            {session.error && (
              <CSDStack direction="row" spacing={2} alignItems="center">
                <CSDTypography variant="body2" color="text.secondary" sx={{ width: 120 }}>
                  Error:
                </CSDTypography>
                <CSDTypography color="error">{session.error}</CSDTypography>
              </CSDStack>
            )}
          </CSDStack>
        </CSDPaper>

        {/* Messages */}
        <CSDPaper sx={{ p: 3 }}>
          <CSDTypography variant="h6" sx={{ mb: 2 }}>
            {t('sessions.messages')} ({session.messages?.length || 0})
          </CSDTypography>
          <CSDStack spacing={2} sx={{ maxHeight: 500, overflow: 'auto' }}>
            {session.messages?.map((msg: Message) => (
              <CSDBox
                key={msg.id}
                sx={{
                  p: 2,
                  borderRadius: 2,
                  bgcolor: msg.role === 'user' ? 'primary.50' : 'grey.100',
                  borderLeft: '3px solid',
                  borderColor: msg.role === 'user' ? 'primary.main' : 'secondary.main',
                }}
              >
                <CSDStack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
                  <CSDChip
                    size="small"
                    label={msg.role}
                    color={msg.role === 'user' ? 'primary' : 'secondary'}
                    variant="outlined"
                  />
                  <CSDTypography variant="caption" color="text.secondary">
                    {formatDateTime(msg.timestamp)}
                  </CSDTypography>
                </CSDStack>
                <CSDTypography
                  variant="body2"
                  sx={{
                    whiteSpace: 'pre-wrap',
                    fontFamily: msg.role === 'assistant' ? 'inherit' : 'monospace',
                  }}
                >
                  {msg.content}
                </CSDTypography>
              </CSDBox>
            ))}
            {(!session.messages || session.messages.length === 0) && (
              <CSDTypography color="text.secondary" sx={{ textAlign: 'center', py: 4 }}>
                No messages in this session
              </CSDTypography>
            )}
          </CSDStack>
        </CSDPaper>
      </CSDStack>

      {/* Terminal Dialog - uses csd-core's terminal infrastructure */}
      {/* Session name/prefix is resolved server-side via token validation */}
      {/* The frontend never sees the actual session name or prefix */}
      {terminalOpen && (
        <CSDTerminalDialog
          open={terminalOpen}
          onClose={handleCloseTerminal}
          nodeId="backend"
          nodeName={`Claude: ${session.name}`}
          nodeType="BACKEND"
          terminalToken={terminalToken || undefined}
          tokenValidateUrl={terminalToken ? `${window.location.origin}/devtrack/api/terminal/validate` : undefined}
        />
      )}
    </CSDLayoutPage>
  );
};

export default SessionDetailPage;
