import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  CSDLayoutPage,
  CSDPaper,
  CSDTypography,
  CSDStack,
  CSDBox,
  CSDIcon,
  CSDButton,
  CSDChip,
} from 'csd_core/UI';
import { useTranslation } from '../../../translations/TranslationContext';
import { useGraphQL } from '../../../shared/hooks';

interface ProjectDetail {
  id: string;
  name: string;
  path: string;
  type: string;
  gitBranch: string;
  gitDirty: boolean;
  components: Record<string, {
    type: string;
    path: string;
    enabled: boolean;
  }>;
}

const ProjectDetailPage: React.FC = () => {
  const { t } = useTranslation();
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { request } = useGraphQL();
  const [project, setProject] = useState<ProjectDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadData = async () => {
      if (!id) return;
      try {
        setLoading(true);
        const result = await request<{ project: ProjectDetail }>(`
          query($id: String!) {
            project(id: $id) { id name path type gitBranch gitDirty components }
          }
        `, { id });
        setProject(result.project);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load project');
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, [id, request]);

  if (loading) {
    return (
      <CSDLayoutPage title={t('common.loading')}>
        <CSDBox sx={{ p: 3, textAlign: 'center' }}>
          <CSDTypography>{t('common.loading')}</CSDTypography>
        </CSDBox>
      </CSDLayoutPage>
    );
  }

  if (error || !project) {
    return (
      <CSDLayoutPage title={t('common.error')}>
        <CSDBox sx={{ p: 3, textAlign: 'center' }}>
          <CSDTypography color="error">{error || 'Project not found'}</CSDTypography>
          <CSDButton onClick={() => navigate('/devtrack/projects')} sx={{ mt: 2 }}>
            {t('common.back')}
          </CSDButton>
        </CSDBox>
      </CSDLayoutPage>
    );
  }

  return (
    <CSDLayoutPage
      title={project.name}
      breadcrumbs={[
        { label: t('projects.title'), path: '/devtrack/projects' },
        { label: project.name },
      ]}
      actions={
        <CSDStack direction="row" spacing={1}>
          <CSDButton variant="outlined" startIcon={<CSDIcon>terminal</CSDIcon>}>
            Terminal
          </CSDButton>
          <CSDButton variant="contained" startIcon={<CSDIcon>build</CSDIcon>}>
            Build
          </CSDButton>
        </CSDStack>
      }
    >
      <CSDStack spacing={3}>
        {/* Project Info */}
        <CSDPaper sx={{ p: 3 }}>
          <CSDStack spacing={2}>
            <CSDStack direction="row" spacing={2} alignItems="center">
              <CSDTypography variant="body2" color="text.secondary" sx={{ width: 100 }}>
                Path:
              </CSDTypography>
              <CSDTypography variant="body1" fontFamily="monospace">
                {project.path}
              </CSDTypography>
            </CSDStack>
            <CSDStack direction="row" spacing={2} alignItems="center">
              <CSDTypography variant="body2" color="text.secondary" sx={{ width: 100 }}>
                Type:
              </CSDTypography>
              <CSDChip label={project.type} size="small" />
            </CSDStack>
            <CSDStack direction="row" spacing={2} alignItems="center">
              <CSDTypography variant="body2" color="text.secondary" sx={{ width: 100 }}>
                Branch:
              </CSDTypography>
              <CSDStack direction="row" alignItems="center" spacing={1}>
                <CSDIcon fontSize="small">git_branch</CSDIcon>
                <CSDTypography>{project.gitBranch || 'N/A'}</CSDTypography>
                <CSDChip
                  size="small"
                  label={project.gitDirty ? t('projects.dirty') : t('projects.clean')}
                  color={project.gitDirty ? 'warning' : 'success'}
                />
              </CSDStack>
            </CSDStack>
          </CSDStack>
        </CSDPaper>

        {/* Components */}
        <CSDPaper sx={{ p: 3 }}>
          <CSDTypography variant="h6" sx={{ mb: 2 }}>
            {t('projects.components')}
          </CSDTypography>
          <CSDStack spacing={1}>
            {project.components && Object.entries(project.components).map(([key, comp]) => (
              <CSDBox
                key={key}
                sx={{
                  p: 2,
                  borderRadius: 1,
                  bgcolor: comp.enabled ? 'background.paper' : 'action.disabledBackground',
                  border: '1px solid',
                  borderColor: 'divider',
                }}
              >
                <CSDStack direction="row" justifyContent="space-between" alignItems="center">
                  <CSDStack direction="row" spacing={2} alignItems="center">
                    <CSDIcon color={comp.enabled ? 'primary' : 'disabled'}>
                      {comp.type === 'frontend' ? 'web' :
                       comp.type === 'backend' ? 'dns' :
                       comp.type === 'cli' ? 'terminal' : 'code'}
                    </CSDIcon>
                    <CSDStack>
                      <CSDTypography fontWeight="medium">{comp.type}</CSDTypography>
                      <CSDTypography variant="body2" color="text.secondary" fontFamily="monospace">
                        {comp.path}
                      </CSDTypography>
                    </CSDStack>
                  </CSDStack>
                  <CSDChip
                    size="small"
                    label={comp.enabled ? 'Enabled' : 'Disabled'}
                    color={comp.enabled ? 'success' : 'default'}
                  />
                </CSDStack>
              </CSDBox>
            ))}
          </CSDStack>
        </CSDPaper>
      </CSDStack>
    </CSDLayoutPage>
  );
};

export default ProjectDetailPage;
