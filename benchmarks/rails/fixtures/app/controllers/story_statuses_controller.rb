class StoryStatusesController < ApplicationController
  def index
    @statuses = StoryStatus.all
  end

  def show
    @status = StoryStatus.find(params[:id])
  end

  def new
    @status = StoryStatus.new
  end

  def create
    @status = StoryStatus.create(status_params)
    NotificationService.notify("status_created", @status)
    redirect_to @status
  end

  def edit
    @status = StoryStatus.find(params[:id])
  end

  def update
    @status = StoryStatus.find(params[:id])
    if @status.update(status_params)
      redirect_to @status
    else
      render :edit
    end
  end

  def destroy
    @status = StoryStatus.find(params[:id])
    @status.destroy
    redirect_to story_statuses_path
  end

  private

  def status_params
    params.require(:story_status).permit(:name, :color)
  end
end
