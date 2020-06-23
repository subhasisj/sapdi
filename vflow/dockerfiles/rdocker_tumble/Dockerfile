#FROM opensuse/leap:15.1
#FROM opensuse/leap:15.0
FROM opensuse/tumbleweed

RUN zypper --non-interactive update && \
    # Install tar, gzip, python, python3, pip, pip3, gcc and libgthread
    zypper --non-interactive install --no-recommends --force-resolution \
#   make \
    java-1_8_0-openjdk \
    vim \
    tar \
    gzip \
    ncompress \   
    python3 \
    python3-pip \
    gcc \
    gcc-c++ \
    libgthread-2_0-0=2.64.2 && \
    # Install tornado  
    python3 -m pip --no-cache install tornado==5.0.2

RUN zypper --non-interactive update \
               && zypper --non-interactive install --no-recommends --force-resolution \
							  R-base>=3.6 R-base-devel \
                              curl \
        && echo 'options(repos = c(CRAN = "https://cran.rstudio.com/"), download.file.method = "libcurl")' \ 
               && rm -rf /tmp/downloaded_packages/ /tmp/*.rds \
               && rm -rf /var/lib/apt/lists/*


RUN zypper --non-interactive update
RUN zypper in -y gcc-fortran
RUN zypper in -y gcc-c++
RUN zypper install -y libcurl-devel \
					  openssl-devel \
					  libxml2-devel \
					  postgresql-devel \
					  libmysqlclient-devel \
					  fontconfig-devel \
					  pkg-config \
					  cairo \
					  libcairo2  \
					  shadow \
					  libpng12-0 \
                      libpng16-compat-devel \
                      libjpeg62-devel-32bit-62.3.0-58.1.x86_64





# install.packages( c( 'haven' 'curl' , 'rio, 'car', 'AER'), dependencies=TRUE)

#RUN Rscript -e 'install.packages("Rserve", repo="http://cran.rstudio.com/", dependencies=TRUE)'
#RUN Rscript -e 'install.packages('Rserve',,"http://rforge.net/",type="source")'
RUN Rscript -e 'install.packages("Rserve",,"http://rforge.net")'
RUN Rscript -e 'install.packages("blorr", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("glmnet", repo="http://cran.rstudio.com/", dependencies=TRUE)'
#RUN Rscript -e 'install.packages("tree", repo="http://cran.rstudio.com/", dependencies=TRUE)'


RUN Rscript -e 'install.packages("devtools", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("jsonlite", repo="http://cran.rstudio.com/", dependencies=TRUE)'
#RUN Rscript -e 'install.packages("tree", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("dplyr", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("caret", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("randomForest", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("Metrics", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("pROC", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("e1071", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("rpart", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("multcomp", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("partykit", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("party", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("readxl", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("stats", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("Hmisc", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("onehot", repo="http://cran.rstudio.com/", dependencies=TRUE)'

RUN Rscript -e 'install.packages("tidyverse", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("DataExplorer", repo="http://cran.rstudio.com/", dependencies=TRUE)'

#RUN Rscript -e 'install.packages("blorr", repo="http://cran.rstudio.com/", dependencies=TRUE)'


#new packages for ubp
RUN Rscript -e 'install.packages("data.table", repo="http://cran.rstudio.com/", dependencies=TRUE)'

RUN Rscript -e 'install.packages("ggplot2", repo="http://cran.rstudio.com/", dependencies=TRUE)'
#RUN Rscript -e 'install.packages("glmnet", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("haven", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("LDPD", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("logistf", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("lsr", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("lubridate", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("Matrix", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("openxlsx", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("PRROC", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("rpart.plot", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("smbinning", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("sqldf", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("stringi", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("svDialogs", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("tidyr", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("tree", repo="http://cran.rstudio.com/", dependencies=TRUE)'
RUN Rscript -e 'install.packages("zoo", repo="http://cran.rstudio.com/", dependencies=TRUE)'


##
RUN Rscript -e 'install.packages("https://cran.rstudio.com/src/contrib/Archive/msgpack/msgpack_1.0.tar.gz",repos=NULL, method="libcurl")'
#RUN Rscript -e 'install.packages("utils", repo="http://cran.rstudio.com/", dependencies=TRUE)'

# Below command throws error in local opensuse linux box also.
#RUN Rscript -e 'install.packages('https://cran.r-project.org/src/contrib/Archive/msgpack/msgpack_1.0.tar.gz', repos=NULL, method='libcurl')'

RUN echo "maxinbuf 4194304" > /etc/Rserve.conf

RUN groupadd -g 1972 vflow && useradd -g 1972 -u 1972 -m vflow
USER 1972:1972

WORKDIR /home/vflow
ENV HOME=/home/vflow
