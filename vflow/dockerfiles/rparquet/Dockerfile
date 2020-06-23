FROM opensuse/tumbleweed

RUN zypper --non-interactive update \
               && zypper --non-interactive install --no-recommends --force-resolution \
							  R-base>=3.6 R-base-devel \
                              curl \
        && echo 'options(repos = c(CRAN = "https://cran.rstudio.com/"), download.file.method = "libcurl")' \ 
               && rm -rf /tmp/downloaded_packages/ /tmp/*.rds \
               && rm -rf /var/lib/apt/lists/*
	
RUN zypper --non-interactive update && \
    # Install tar, gzip, python, python3, pip, pip3, gcc and libgthread
    zypper --non-interactive install --no-recommends --force-resolution \
    # make \
    #java-1_8_0-openjdk \
    # vim \
    # tar \
    # gzip \
	#git-core \
    # ncompress \ 
    python3 \
    python3-pip 
    # gcc=7 \
    # gcc-c++=7 \ 
    # libgthread-2_0-0=2.54.3
	
RUN zypper --non-interactive in git-core

WORKDIR /home/vflow
ENV HOME=/home/vflow
RUN git clone https://github.com/subhasisj/Credit_Analysis_R.git $HOME

RUN Rscript -e 'install.packages("randomForest", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("data.table", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("caret", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("devtools", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("jsonlite", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("Rserve",,"http://rforge.net")'
RUN Rscript -e 'install.packages("arrow",,"http://rforge.net")'

RUN echo "maxinbuf 4194304" > /etc/Rserve.conf
RUN groupadd -g 1972 vflow && useradd -g 1972 -u 1972 -m vflow
USER 1972:1972


#RUN mkdir /Credit_Analysis_R/
#WORKDIR /Credit_Analysis_R/
#RUN cd /Credit_Analysis_R/
#FROM python:3.6
#RUN git clone https://github.com/subhasisj/Credit_Analysis_R.git 
# CMD ["bin/bash"]