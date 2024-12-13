pipeline {
    agent any
    environment {
        IMAGE_REPO_NAME = "rencana-kerja-cron"
        BUCKET_NAME = "gobimbel-spin"
        HELM_STORE_PATH = "values/rencana-kerja-cron"
        AWS_CREDENTIALS_ID = "jenkins-aws"
        APP_NAME="rencana-kerja-cron"
    }
    stages {
        stage('Clone repository') {
            steps {
                script {
                    try {
                        checkout scm
                    } catch (Exception e) {
                        BUILD_ERROR = "Failed to clone repository: ${e.message}"
                        error "Failed to clone repository: ${e.message}"
                    }
                }
            }
        }

        stage('Code Scan'){
            when {
                anyOf {
                    branch 'main'
                    branch 'staging'
                }
            }
            environment {
                scannerHome = tool 'sonar-scanner'
            }
            steps {
                script {
                    try {
                        withSonarQubeEnv('sonar-staging'){
                            sh "${scannerHome}/bin/sonar-scanner -Dsonar.plugins.downloadOnlyRequired=true -Dsonar.projectKey=${APP_NAME} -Dsonar.exclusions=node_modules/**,dist/**,helm/**,deploy/**,Jenkinsfile -Dsonar.languages=ts"
                        }
                    } catch (Exception e) {
                        BUILD_ERROR = "Failed to scanning source code: ${e.message}"
                        error "Failed to scanning source code: ${e.message}"
                    }
                }
            }
        }

        stage('Push values to S3') {
            when {
                anyOf {
                    branch 'master'
                    branch 'staging'
                }
                changeset 'helm/**/**/*.yaml'
            }
            steps {
                script {
                    try {
                        withAWS(region: "ap-southeast-1", credentials: "${AWS_CREDENTIALS_ID}") {
                            s3Upload(file: './helm', bucket: "${BUCKET_NAME}", path: "${HELM_STORE_PATH}")
                        }
                    } catch (Exception e) {
                        BUILD_ERROR = "Failed to upload to S3: ${e.message}"
                        error "Failed to upload to S3: ${e.message}"
                    }
                }
            }
        }

        stage('Building Image') {
            when {
                anyOf {
                    branch 'staging'
                    buildingTag()
                }
            }
            steps {
                script {
                    container('dind') {
                        try {
                            app = docker.build("${IMAGE_REPO_NAME}")
                        } catch (Exception e) {
                            BUILD_ERROR = "Failed to build Docker image: ${e.message}"
                            error "Failed to build Docker image: ${e.message}"
                        }
                    }
                }
            }
        }

        stage('Push Image to Repository') {
            when {
                anyOf {
                    branch 'staging'
                    buildingTag()
                }
            }
            steps {
                script {
                    container('dind') {
                        try {
                            pushDockerImage(app)
                        } catch (Exception e) {
                            BUILD_ERROR = "Failed to push Docker image to repository: ${e.message}"
                            error "Failed to push Docker image to repository: ${e.message}"
                        }
                    }
                }
            }
        }
    }

    post{
        always {
            script {
                CONSOLE_LOG = "${env.BUILD_URL}/console"
                BUILD_STATUS = currentBuild.currentResult
                BUILD_ERROR = (BUILD_STATUS == 'SUCCESS') ? "NA" : BUILD_ERROR
                
                // Get today's date in the desired format
                def today = sh(script: 'date +"%b %d"', returnStdout: true).trim()

                sh """
                    sed -i 's|%%JOBNAME%%|${env.JOB_NAME}|g' report.html
                    sed -i 's|%%BUILDNO%%|${env.BUILD_NUMBER}|g' report.html
                    sed -i 's|%%DATE%%|${today}|g' report.html
                    sed -i 's|%%BUILD_STATUS%%|${BUILD_STATUS}|g' report.html
                    sed -i 's|%%ERROR%%|${BUILD_ERROR}|g' report.html
                    sed -i 's|%%CONSOLE_LOG%%|${CONSOLE_LOG}|g' report.html
                """
            }
            publishHTML(target: [
                allowMissing: true,
                alwaysLinkToLastBuild: true,
                keepAll: true,
                reportDir: "${WORKSPACE}",
                reportFiles: 'report.html',
                reportName: 'Build-Report',
                reportTitles: 'Build-Report'
            ])
            sendSlackNotification()
        }
    }
}



def pushDockerImage(app) {
    withCredentials([string(credentialsId: 'AWS_ACCOUNT_ID', variable: 'AWS_ACCOUNT_ID')]) {
        def tagName = env.BRANCH_NAME == 'staging' ? 'staging' : (buildingTag() ? "${TAG_NAME}" : 'latest')
        docker.withRegistry("https://${AWS_ACCOUNT_ID}.dkr.ecr.ap-southeast-1.amazonaws.com", "ecr:ap-southeast-1:${AWS_CREDENTIALS_ID}") {
            app.push(tagName)        }
    }
}

def sendSlackNotification() {
    def status = currentBuild.currentResult
    def color = (status == "SUCCESS") ? 'good' : 'danger'
    def errorDescription = (status == "FAILURE") ? "\n Error description: *${BUILD_ERROR}*" : ""
    
    def buildSummary = "Job: ${env.JOB_NAME}\n Status: *${status}*${errorDescription}\n Build Report: ${env.BUILD_URL}Build-Report"
    
    slackSend(color: color, message: buildSummary, channel: '#cicd')
}