pipeline {
    agent any
    stages {
        stage('test') {
            steps {
                make test
            }
        }
        stage('build') {
            steps {
                make docker-build
            }
        }
        stage('deploy') {
            steps {
                make deploy
            }
        }
    }
}