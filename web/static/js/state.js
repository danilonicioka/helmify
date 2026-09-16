        // State variables
        let chartType = 'single'; // 'single' or 'multi'
        let globalConfig = {
            'TZ': 'America/Belem'
        };
        let globalSecret = {};

        // Components dictionary
        let components = {};

        // Active component name in form view
        let activeComponent = '';

        let generatedFiles = {};
        let activeFile = 'values.yaml';
        let isFirstLoad = true;
        let templatesFolderOpen = true;
